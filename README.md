# 民族乐器调弦口述史证据复核台（task287-tuneproof）

面向民族音乐学研究者的调弦证据复核服务：把口述调弦说明、录音实测音高与乐器构造统一到"音分"刻度，验证调弦关系的物理可行性，裁决地区变体并发布不可变调弦版本快照。

## 业务闭环

1. 创建研究批次 → 2. 录入乐器与弦位构造 → 3. 导入口述片段并附加录音音高观测（指纹幂等）→ 4. 术语对齐（地区变体/标准音程）→ 5. 创建调弦关系并执行可行性检查（音程比对 + 弦位约束）→ 6. 研究者裁决（确认/否决）→ 7. 发布并冻结调弦版本快照。

## 标准命令

```bash
# 启动服务
CGO_ENABLED=0 GOTOOLCHAIN=local go run ./cmd/tuneproof --addr :8080 --db tuneproof.db

# 自检（创建数据→检查→裁决→冻结→重启恢复验证，退出码 0 为通过）
CGO_ENABLED=0 GOTOOLCHAIN=local go run ./cmd/tuneproof --smoke-test

# 构建门禁
CGO_ENABLED=0 GOTOOLCHAIN=local go build ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go vet ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go test ./...
```

## HTTP API（前缀 /api）

| 资源 | 端点 |
|---|---|
| 批次 | `POST /api/batches`、`GET /api/batches`、`GET /api/batches/{id}`、`PATCH /api/batches/{id}/status`、`POST /api/batches/{id}/seal` |
| 乐器/弦位 | `POST /api/instruments`、`GET /api/instruments?batch_id=`、`GET /api/instruments/{id}`、`POST /api/instruments/{id}/strings`、`GET /api/instruments/{id}/strings` |
| 片段/音高 | `POST /api/segments`、`GET /api/segments?batch_id=`、`GET /api/segments/{id}`、`POST /api/segments/{id}/pitch`、`PATCH /api/segments/{id}/status` |
| 对齐 | `POST /api/align/terms`、`GET /api/align/mappings`、`POST /api/align/mappings` |
| 关系 | `POST /api/relations`、`GET /api/relations?batch_id=`、`GET /api/relations/{id}`、`POST /api/relations/{id}/check`、`POST /api/relations/{id}/adjudicate`、`GET /api/relations/conflicts` |
| 变体 | `GET /api/variants`、`POST /api/variants/{id}/adjudicate` |
| 版本 | `POST /api/versions`、`GET /api/versions?batch_id=`、`GET /api/versions/{id}`、`GET /api/versions/{id}/diff`、`POST /api/versions/{id}/supersede`、`POST /api/versions/{id}/freeze`、`POST /api/versions/{id}/share` |
| 统计/健康 | `GET /api/stats`、`GET /api/health` |

## 状态机

- 研究批次：`draft → aligning → reviewing → published → sealed`
- 证据片段：`raw → aligned / ambiguous → excluded`
- 调弦关系：`candidate → feasible / interval_conflict / string_conflict → confirmed / rejected`
- 调弦版本：`draft → shared → frozen → superseded`

## 关键不变量

- 证据片段按内容指纹唯一（幂等导入）；同一关系裁决走乐观锁（version 校验）。
- 拒绝自配对、越界弦位、超范围频率、重复弦位；封存批次与冻结版本后拒绝一切修改。
- 冻结版本快照不可变，仅可被新版本替代（保留 superseded_by 出处）。
