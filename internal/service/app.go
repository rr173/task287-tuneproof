// Package service 编排层：串联 evidence/structure/align/check 各业务包，
// 提供 HTTP 层可直接调用的应用接口与版本/统计能力。
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"task287-tuneproof/internal/align"
	"task287-tuneproof/internal/check"
	"task287-tuneproof/internal/evidence"
	"task287-tuneproof/internal/model"
	"task287-tuneproof/internal/store"
	"task287-tuneproof/internal/structure"
)

// App 应用根对象，持有数据库与各业务服务。
type App struct {
	db        *store.DB
	Evidence  *evidence.Service
	Structure *structure.Service
	Align     *align.Service
	Check     *check.Service
	Batches   *store.BatchStore
	Instruments *store.InstrumentStore
	Segments  *store.SegmentStore
	Mappings  *store.MappingStore
	Relations *store.RelationStore
	Versions  *store.VersionStore
	Audit     *store.AuditStore
}

// New 构造应用：注入 store 实现到各业务服务。
func New(db *store.DB) *App {
	batches := store.NewBatchStore(db)
	instruments := store.NewInstrumentStore(db)
	segments := store.NewSegmentStore(db)
	mappings := store.NewMappingStore(db)
	relations := store.NewRelationStore(db)
	versions := store.NewVersionStore(db)
	audit := store.NewAuditStore(db)

	return &App{
		db:          db,
		Evidence:    evidence.NewService(segments, batches, audit),
		Structure:   structure.NewService(instruments, batches, audit),
		Align:       align.NewService(mappings, audit),
		Check:       check.NewService(relations, segments, instruments, batches, audit),
		Batches:     batches,
		Instruments: instruments,
		Segments:    segments,
		Mappings:    mappings,
		Relations:   relations,
		Versions:    versions,
		Audit:       audit,
	}
}

// DB 暴露底层数据库（供测试与工具使用）。
func (a *App) DB() *store.DB { return a.db }

// CreateVersion 发布调弦版本：把批次当前已确认关系固化为快照。
// 返回创建的版本（草稿态）；调用方随后可共享/冻结。
func (a *App) CreateVersion(batchID int64, name string) (*model.TuningVersion, error) {
	batch, err := a.Batches.Get(batchID)
	if err != nil {
		return nil, err
	}
	if model.BatchClosed(batch.Status) {
		return nil, model.ErrFrozen
	}
	relations, err := a.Relations.List(batchID, model.RelationConfirmed)
	if err != nil {
		return nil, err
	}
	if len(relations) == 0 {
		return nil, model.NewValidationError("relations", "no confirmed relations to snapshot")
	}
	payload, err := json.Marshal(relations)
	if err != nil {
		return nil, fmt.Errorf("marshal snapshot: %w", err)
	}
	v := &model.TuningVersion{
		BatchID:      batchID,
		Name:         name,
		Status:       model.VersionDraft,
		SnapshotJSON: string(payload),
		Fingerprint:  snapshotFingerprint(payload),
	}
	id, err := a.Versions.Create(v)
	if err != nil {
		return nil, err
	}
	_ = a.Audit.Log(batchID, "version.create", "tuning_version", id, name)
	return a.Versions.Get(id)
}

// FreezeVersion 冻结版本：草稿/共享 → 冻结（不可变快照）。
func (a *App) FreezeVersion(ctx context.Context, id int64, expectedVersion int64) (*model.TuningVersion, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	v, err := a.Versions.Get(id)
	if err != nil {
		return nil, err
	}
	if model.VersionImmutable(v.Status) {
		return nil, model.ErrFrozen
	}
	if !model.ValidVersionTransition(v.Status, model.VersionFrozen) {
		return nil, model.ErrInvalidState
	}
	frozen, err := a.Versions.UpdateStatus(id, model.VersionFrozen, 0, expectedVersion)
	if err != nil {
		return nil, err
	}
	_ = a.Audit.Log(v.BatchID, "version.freeze", "tuning_version", id, v.Name)
	return frozen, nil
}

// ShareVersion 共享版本：草稿 → 共享。
func (a *App) ShareVersion(id int64, expectedVersion int64) (*model.TuningVersion, error) {
	v, err := a.Versions.Get(id)
	if err != nil {
		return nil, err
	}
	if model.VersionImmutable(v.Status) {
		return nil, model.ErrFrozen
	}
	if !model.ValidVersionTransition(v.Status, model.VersionShared) {
		return nil, model.ErrInvalidState
	}
	shared, err := a.Versions.UpdateStatus(id, model.VersionShared, 0, expectedVersion)
	if err != nil {
		return nil, err
	}
	_ = a.Audit.Log(v.BatchID, "version.share", "tuning_version", id, v.Name)
	return shared, nil
}

// SupersedeVersion 替代版本：冻结版本被新版本替代（保留出处）。
func (a *App) SupersedeVersion(id int64, newID int64, expectedVersion int64) (*model.TuningVersion, error) {
	v, err := a.Versions.Get(id)
	if err != nil {
		return nil, err
	}
	if v.Status != model.VersionFrozen {
		return nil, model.ErrInvalidState // 只允许冻结版本被替代
	}
	if _, err := a.Versions.Get(newID); err != nil {
		return nil, err
	}
	sup, err := a.Versions.UpdateStatus(id, model.VersionSuperseded, newID, expectedVersion)
	if err != nil {
		return nil, err
	}
	_ = a.Audit.Log(v.BatchID, "version.supersede", "tuning_version", id, fmt.Sprintf("by %d", newID))
	return sup, nil
}

// VersionDiff 对比两个版本快照：返回新增/移除的关系 ID。
func (a *App) VersionDiff(oldID, newID int64) (added, removed []int64, err error) {
	oldV, err := a.Versions.Get(oldID)
	if err != nil {
		return nil, nil, err
	}
	newV, err := a.Versions.Get(newID)
	if err != nil {
		return nil, nil, err
	}
	oldSet, err := snapshotIDs(oldV.SnapshotJSON)
	if err != nil {
		return nil, nil, err
	}
	newSet, err := snapshotIDs(newV.SnapshotJSON)
	if err != nil {
		return nil, nil, err
	}
	for id := range newSet {
		if !oldSet[id] {
			added = append(added, id)
		}
	}
	for id := range oldSet {
		if !newSet[id] {
			removed = append(removed, id)
		}
	}
	return added, removed, nil
}

// Stats 统计总览。
type Stats struct {
	BatchCount        int `json:"batch_count"`
	InstrumentCount   int `json:"instrument_count"`
	SegmentCount      int `json:"segment_count"`
	RelationCount     int `json:"relation_count"`
	FeasibleCount     int `json:"feasible_count"`
	ConflictCount     int `json:"conflict_count"`
	ConfirmedCount    int `json:"confirmed_count"`
	RejectedCount     int `json:"rejected_count"`
	VersionCount      int `json:"version_count"`
	FrozenVersionCount int `json:"frozen_version_count"`
	AmbiguousSegmentCount int `json:"ambiguous_segment_count"`
}

// Stats 统计全库。
func (a *App) Stats() (*Stats, error) {
	s := &Stats{}
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM research_batches`).Scan(&s.BatchCount)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM instruments`).Scan(&s.InstrumentCount)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM evidence_segments`).Scan(&s.SegmentCount)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM tuning_relations`).Scan(&s.RelationCount)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM tuning_relations WHERE status = ?`, model.RelationFeasible).Scan(&s.FeasibleCount)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM tuning_relations WHERE status IN (?,?)`,
		model.RelationIntervalConflict, model.RelationStringConflict).Scan(&s.ConflictCount)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM tuning_relations WHERE status = ?`, model.RelationConfirmed).Scan(&s.ConfirmedCount)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM tuning_relations WHERE status = ?`, model.RelationRejected).Scan(&s.RejectedCount)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM tuning_versions`).Scan(&s.VersionCount)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM tuning_versions WHERE status = ?`, model.VersionFrozen).Scan(&s.FrozenVersionCount)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM evidence_segments WHERE status = ?`, model.SegmentAmbiguous).Scan(&s.AmbiguousSegmentCount)
	return s, nil
}

// snapshotFingerprint 快照指纹：SHA-256。
func snapshotFingerprint(payload []byte) string {
	h := sha256.Sum256(payload)
	return hex.EncodeToString(h[:])
}

// snapshotIDs 从快照 JSON 提取关系 ID 集合。
func snapshotIDs(snapshot string) (map[int64]bool, error) {
	var rels []*model.TuningRelation
	if err := json.Unmarshal([]byte(snapshot), &rels); err != nil {
		return nil, fmt.Errorf("unmarshal snapshot: %w", err)
	}
	set := make(map[int64]bool, len(rels))
	for _, r := range rels {
		set[r.ID] = true
	}
	return set, nil
}

// Now 提供给外部使用的当前时间（对齐 store 时间格式）。
func Now() time.Time { return time.Now().UTC() }
