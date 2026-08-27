package store

import (
	"fmt"

	"task287-tuneproof/internal/model"
)

// AuditStore 审计日志存取。
type AuditStore struct{ db *DB }

// NewAuditStore 构造审计存储。
func NewAuditStore(db *DB) *AuditStore { return &AuditStore{db: db} }

// Log 追加一条审计记录。
func (s *AuditStore) Log(batchID int64, action, entity string, entityID int64, detail string) error {
	_, err := s.db.Exec(
		`INSERT INTO audit_logs(batch_id, action, entity, entity_id, detail, created_at)
		 VALUES(?,?,?,?,?,?)`,
		batchID, action, entity, entityID, detail, Now())
	if err != nil {
		return fmt.Errorf("insert audit: %w", err)
	}
	return nil
}

// List 按批次列出审计记录。
func (s *AuditStore) List(batchID int64) ([]*model.AuditLog, error) {
	rows, err := s.db.Query(
		`SELECT id, batch_id, action, entity, entity_id, detail, created_at
		 FROM audit_logs WHERE batch_id = ? ORDER BY id DESC`, batchID)
	if err != nil {
		return nil, fmt.Errorf("list audit: %w", err)
	}
	defer rows.Close()
	var out []*model.AuditLog
	for rows.Next() {
		var a model.AuditLog
		var created string
		if err := rows.Scan(&a.ID, &a.BatchID, &a.Action, &a.Entity, &a.EntityID, &a.Detail, &created); err != nil {
			return nil, err
		}
		a.CreatedAt, _ = parseTime(created)
		out = append(out, &a)
	}
	return out, rows.Err()
}
