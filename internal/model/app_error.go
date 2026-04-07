package model

import "fmt"

const (
	ErrCodeInvalidJSON       = "INVALID_JSON"
	ErrCodeInvalidModelJSON  = "INVALID_MODEL_JSON"
	ErrCodeEmptyLogText      = "EMPTY_LOG_TEXT"
	ErrCodeLogTooLong        = "LOG_TOO_LONG"
	ErrCodeInvalidBattleType = "INVALID_BATTLE_TYPE"
	ErrCodeInvalidBuildTags  = "INVALID_BUILD_TAGS"
	ErrCodeNotesTooLong      = "NOTES_TOO_LONG"
	ErrCodeInvalidArgument   = "INVALID_ARGUMENT"
	ErrCodeAnalyzeFailed     = "ANALYZE_FAILED"
	ErrCodeInternalError     = "INTERNAL_ERROR"
)

type AppError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	if e.Code == "" {
		return e.Message
	}
	if e.Message == "" {
		return e.Code
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}
