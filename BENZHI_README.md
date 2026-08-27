# BENZHI 评测说明

基于 Go 实现的民族乐器调弦口述史证据复核 Web 项目，一款后端服务，完成口述调弦术语与录音音高的对齐、调弦关系可行性的音程比对与弦位约束检查、地区变体裁决与不可变调弦版本快照发布。

## 启动

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go run ./cmd/tuneproof --addr :8080 --db tuneproof.db
```

## 自检（不启动长驻服务）

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go run ./cmd/tuneproof --smoke-test
```

## 构建门禁

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go build ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go vet ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go test ./...
```

## HTTP API

路由统一 `/api` 前缀：研究批次创建/推进/封存、乐器与弦位录入、口述片段导入（指纹幂等）与音高观测附加、术语对齐与映射维护、调弦关系创建/可行性检查/裁决、变体候选裁决、版本发布/共享/冻结/替代/差异对比、统计与健康检查。

## 持久化

SQLite（modernc.org/sqlite 纯 Go 驱动），保存研究批次、乐器弦位、证据片段、音高观测、术语映射、调弦关系、变体候选与调弦版本；重启后恢复对齐与裁决状态，证据片段按指纹幂等，冻结版本保留完整快照与出处。
