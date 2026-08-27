package model

import "errors"

// 领域错误统一出口：store/service/httpapi 三层共用。
var (
	// ErrNotFound 实体不存在。
	ErrNotFound = errors.New("entity not found")
	// ErrConflict 版本冲突（乐观锁）：并发修改同一实体。
	ErrConflict = errors.New("version conflict")
	// ErrInvalidState 非法状态流转。
	ErrInvalidState = errors.New("invalid state transition")
	// ErrFrozen 冻结对象不可修改。
	ErrFrozen = errors.New("entity is frozen")
	// ErrDuplicate 唯一键重复（弦位重复、指纹重复）。
	ErrDuplicate = errors.New("duplicate entity")
	// ErrValidation 输入校验失败。
	ErrValidation = errors.New("validation failed")
	// ErrRelation 关系不成立（自配对/越界）。
	ErrRelation = errors.New("invalid relation")
)

// ValidationError 携带具体字段错误的校验失败。
type ValidationError struct {
	Field string
	Msg   string
}

func (e *ValidationError) Error() string {
	return "validation failed on " + e.Field + ": " + e.Msg
}

func (e *ValidationError) Unwrap() error { return ErrValidation }

// NewValidationError 构造校验错误。
func NewValidationError(field, msg string) *ValidationError {
	return &ValidationError{Field: field, Msg: msg}
}
