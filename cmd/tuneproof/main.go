// Command tuneproof 民族乐器调弦口述史证据复核台。
//
// 服务把口述调弦说明、录音音高观测与乐器构造统一到"音分"刻度，
// 验证调弦关系的物理可行性并发布不可变版本快照。
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"task287-tuneproof/internal/evidence"
	"task287-tuneproof/internal/httpapi"
	"task287-tuneproof/internal/model"
	"task287-tuneproof/internal/service"
	"task287-tuneproof/internal/store"
)

func main() {
	var (
		addr      = flag.String("addr", ":8080", "HTTP 监听地址")
		dbPath    = flag.String("db", "tuneproof.db", "SQLite 数据库路径")
		smokeTest = flag.Bool("smoke-test", false, "运行自检并退出（不启动长驻服务）")
	)
	flag.Parse()

	if *smokeTest {
		if err := runSmokeTest(*dbPath); err != nil {
			log.Fatalf("smoke-test 失败: %v", err)
		}
		fmt.Println("smoke-test OK")
		return
	}

	db, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer db.Close()

	app := service.New(db)
	router := httpapi.NewRouter(app)

	srv := &http.Server{
		Addr:              *addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("民族乐器调弦口述史证据复核台 启动于 %s（数据库 %s）", *addr, *dbPath)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP 服务错误: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

// runSmokeTest 自检契约：
// 真实创建批次/乐器/片段/音高/关系，执行对齐与可行性检查，
// 裁决并发布冻结版本，然后关闭并重新打开同一数据库验证持久化与重启恢复，
// 最后校验错误边界（冻结后拒绝修改）。全部通过退出码为 0。
func runSmokeTest(dbPath string) error {
	// 用独立 smoke 库，避免污染正式库
	smokePath := smokeDBPath(dbPath)
	_ = os.Remove(smokePath)

	// ---- 第一段：创建数据 ----
	db, err := store.Open(smokePath)
	if err != nil {
		return fmt.Errorf("打开 smoke 库: %w", err)
	}
	app := service.New(db)

	// 1. 创建研究批次
	batchID, err := app.Batches.Create(&model.ResearchBatch{
		Name:        "江南丝竹调弦口述复核",
		Description: "验证 '高弦下调' 口述与录音音高的一致性",
		Region:      "江南",
	})
	if err != nil {
		db.Close()
		return fmt.Errorf("创建批次: %w", err)
	}

	// 2. 录入三弦乐器（琵琶简化模型）
	insID, err := app.Instruments.Create(&model.Instrument{
		BatchID:     batchID,
		Name:        "示例琵琶",
		Category:    "弹拨",
		Region:      "江南",
		StringCount: 3,
	})
	if err != nil {
		db.Close()
		return fmt.Errorf("录入乐器: %w", err)
	}
	positions := []*model.StringPosition{
		{InstrumentID: insID, Position: 1, Name: "低弦", MinFreqHz: 80, MaxFreqHz: 200, StandardHz: 110},
		{InstrumentID: insID, Position: 2, Name: "中弦", MinFreqHz: 180, MaxFreqHz: 400, StandardHz: 220},
		{InstrumentID: insID, Position: 3, Name: "高弦", MinFreqHz: 300, MaxFreqHz: 700, StandardHz: 440},
	}
	for _, p := range positions {
		if _, err := app.Instruments.AddPosition(p); err != nil {
			db.Close()
			return fmt.Errorf("添加弦位 %d: %w", p.Position, err)
		}
	}

	// 3. 导入口述片段（幂等验证：重复导入返回既有片段）
	transcript := "高弦下调，与中弦成纯四度；低弦与中弦成纯五度。"
	seg, created, err := app.Evidence.ImportSegment(evidence.ImportInput{
		BatchID:    batchID,
		SourceType: "oral_history",
		SourceRef:  "录音 A 第 3 分 12 秒",
		Region:     "江南",
		Transcript: transcript,
	})
	if err != nil {
		db.Close()
		return fmt.Errorf("导入口述片段: %w", err)
	}
	if !created {
		db.Close()
		return fmt.Errorf("首次导入应创建新片段")
	}
	dup, created, err := app.Evidence.ImportSegment(evidence.ImportInput{
		BatchID:    batchID,
		SourceType: "oral_history",
		SourceRef:  "录音 A 第 3 分 12 秒",
		Region:     "江南",
		Transcript: transcript,
	})
	if err != nil {
		db.Close()
		return fmt.Errorf("重复导入: %w", err)
	}
	if created || dup.ID != seg.ID {
		db.Close()
		return fmt.Errorf("幂等导入失败: 重复导入未返回既有片段")
	}
	segID := seg.ID

	// 4. 附加音高观测（模拟录音实测：中弦 220Hz，高弦 330Hz = 纯五度，与"纯四度"描述冲突）
	if _, err := app.Evidence.AttachPitch(context.Background(), &model.PitchObservation{
		SegmentID: segID, StringPos: 1, FrequencyHz: 110, Unit: "hz", Confidence: 0.95,
		RecordedAt: time.Now().UTC(),
	}); err != nil {
		db.Close()
		return fmt.Errorf("附加低弦音高: %w", err)
	}
	if _, err := app.Evidence.AttachPitch(context.Background(), &model.PitchObservation{
		SegmentID: segID, StringPos: 2, FrequencyHz: 220, Unit: "hz", Confidence: 0.95,
		RecordedAt: time.Now().UTC(),
	}); err != nil {
		db.Close()
		return fmt.Errorf("附加中弦音高: %w", err)
	}
	if _, err := app.Evidence.AttachPitch(context.Background(), &model.PitchObservation{
		SegmentID: segID, StringPos: 3, FrequencyHz: 330, Unit: "hz", Confidence: 0.9,
		RecordedAt: time.Now().UTC(),
	}); err != nil {
		db.Close()
		return fmt.Errorf("附加高弦音高: %w", err)
	}

	// 5. 术语对齐
	claim, ok, err := app.Align.AlignTranscript(transcript, "江南")
	if err != nil {
		db.Close()
		return fmt.Errorf("术语对齐: %w", err)
	}
	if !ok {
		db.Close()
		return fmt.Errorf("术语对齐应成功解析")
	}
	if claim.Normalized == "" {
		db.Close()
		return fmt.Errorf("对齐结果缺归一化动作")
	}
	// 片段标记已对齐
	seg2, err := app.Evidence.SetStatus(segID, model.SegmentAligned, seg.Version)
	if err != nil {
		db.Close()
		return fmt.Errorf("片段对齐状态: %w", err)
	}

	// 6. 创建调弦关系：高弦(3)→中弦(2)，口述纯四度 500 音分
	relID, err := app.Relations.Create(&model.TuningRelation{
		BatchID:          batchID,
		InstrumentID:     insID,
		SegmentID:        segID,
		FromPosition:     3,
		ToPosition:       2,
		DescribedTerm:    "纯四度",
		DescribedInterval: 500,
	})
	if err != nil {
		db.Close()
		return fmt.Errorf("创建调弦关系: %w", err)
	}

	// 7. 可行性检查：330/220 = 纯五度 702 音分 vs 描述 500 → 应判音程冲突
	report, err := app.Check.CheckRelation(relID)
	if err != nil {
		db.Close()
		return fmt.Errorf("可行性检查: %w", err)
	}
	if report.Verdict != model.RelationIntervalConflict {
		db.Close()
		return fmt.Errorf("预期音程冲突，实际 %s（实测 %d 音分）", report.Verdict, report.MeasuredCents)
	}
	rel, err := app.Relations.Get(relID)
	if err != nil {
		db.Close()
		return err
	}

	// 8. 研究者修订：转录歧义 → 修正为纯五度 → 重检 → 确认
	if _, err := app.Evidence.SetStatus(segID, model.SegmentAmbiguous, seg2.Version); err != nil {
		db.Close()
		return fmt.Errorf("标记转录歧义: %w", err)
	}
	rel2, err := app.Relations.Adjudicate(relID, model.RelationCandidate, rel.Version)
	if err != nil {
		db.Close()
		return fmt.Errorf("回退候选: %w", err)
	}
	// 修正描述音程为纯五度 700
	corrected, err := app.Relations.UpdateCheckResult(relID, report.MeasuredCents, 702-700, model.RelationFeasible, "修订转录后与实测一致", rel2.Version)
	if err != nil {
		db.Close()
		return fmt.Errorf("修正检查结果: %w", err)
	}
	confirmed, err := app.Relations.Adjudicate(relID, model.RelationConfirmed, corrected.Version)
	if err != nil {
		db.Close()
		return fmt.Errorf("确认关系: %w", err)
	}
	if confirmed.Status != model.RelationConfirmed {
		db.Close()
		return fmt.Errorf("关系应已确认")
	}

	// 9. 推进批次状态并发布冻结版本
	for _, st := range []string{model.BatchAligning, model.BatchReviewing, model.BatchPublished} {
		b, err := app.Batches.Get(batchID)
		if err != nil {
			db.Close()
			return err
		}
		if _, err := app.Batches.UpdateStatus(batchID, st, b.Version); err != nil {
			db.Close()
			return fmt.Errorf("推进批次到 %s: %w", st, err)
		}
	}
	ver, err := app.CreateVersion(batchID, "v1-江南调弦")
	if err != nil {
		db.Close()
		return fmt.Errorf("发布版本: %w", err)
	}
	if ver.Status != model.VersionDraft {
		db.Close()
		return fmt.Errorf("新版本应为草稿")
	}
	frozen, err := app.FreezeVersion(context.Background(), ver.ID, ver.Version)
	if err != nil {
		db.Close()
		return fmt.Errorf("冻结版本: %w", err)
	}
	if frozen.Status != model.VersionFrozen || frozen.Fingerprint == "" {
		db.Close()
		return fmt.Errorf("冻结版本指纹缺失")
	}

	// 10. 错误边界：封存批次后拒绝新增片段
	b, err := app.Batches.Get(batchID)
	if err != nil {
		db.Close()
		return err
	}
	sealed, err := app.Batches.UpdateStatus(batchID, model.BatchSealed, b.Version)
	if err != nil {
		db.Close()
		return fmt.Errorf("封存批次: %w", err)
	}
	if sealed.Status != model.BatchSealed {
		db.Close()
		return fmt.Errorf("批次应已封存")
	}
	if _, _, err := app.Evidence.ImportSegment(evidence.ImportInput{
		BatchID:    batchID,
		SourceType: "oral_history",
		SourceRef:  "X",
		Region:     "江南",
		Transcript: "封存后不应允许导入",
	}); err != model.ErrFrozen {
		db.Close()
		return fmt.Errorf("封存后导入应被拒绝，实际 %v", err)
	}

	// ---- 第二段：重启恢复验证 ----
	db.Close()
	time.Sleep(100 * time.Millisecond)

	rebooted, err := store.Open(smokePath)
	if err != nil {
		return fmt.Errorf("重开数据库: %w", err)
	}
	defer rebooted.Close()
	app2 := service.New(rebooted)

	batch2, err := app2.Batches.Get(batchID)
	if err != nil {
		return fmt.Errorf("重启后批次丢失: %w", err)
	}
	if batch2.Status != model.BatchSealed {
		return fmt.Errorf("重启后批次状态应为封存，实际 %s", batch2.Status)
	}
	rel3, err := app2.Relations.Get(relID)
	if err != nil {
		return fmt.Errorf("重启后关系丢失: %w", err)
	}
	if rel3.Status != model.RelationConfirmed {
		return fmt.Errorf("重启后关系应为确认，实际 %s", rel3.Status)
	}
	seg3, pitches, err := app2.Evidence.Get(segID)
	if err != nil {
		return fmt.Errorf("重启后片段丢失: %w", err)
	}
	if seg3.Fingerprint == "" {
		return fmt.Errorf("重启后片段指纹丢失")
	}
	if len(pitches) < 3 {
		return fmt.Errorf("重启后音高观测丢失: got %d", len(pitches))
	}
	ver2, err := app2.Versions.Get(ver.ID)
	if err != nil {
		return fmt.Errorf("重启后版本丢失: %w", err)
	}
	if ver2.Status != model.VersionFrozen {
		return fmt.Errorf("重启后版本应为冻结，实际 %s", ver2.Status)
	}
	stats, err := app2.Stats()
	if err != nil {
		return fmt.Errorf("重启后统计: %w", err)
	}
	if stats.ConfirmedCount < 1 || stats.FrozenVersionCount < 1 {
		return fmt.Errorf("重启后统计异常: %+v", stats)
	}

	// 清理 smoke 库
	_ = os.Remove(smokePath)
	fmt.Printf("自检通过：批次=%s 关系=%s 版本=%s 冻结=%d 确认=%d\n",
		batch2.Status, rel3.Status, ver2.Status, stats.FrozenVersionCount, stats.ConfirmedCount)
	return nil
}

// smokeDBPath 派生 smoke 库路径（正式库名 + .smoke 后缀）。
func smokeDBPath(dbPath string) string {
	dir, base := filepath.Split(dbPath)
	return filepath.Join(dir, base+".smoke")
}
