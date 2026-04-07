package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/feiai2017/battle_mind/internal/llm"
	"github.com/feiai2017/battle_mind/internal/model"
	"github.com/feiai2017/battle_mind/internal/service"
)

const (
	requestIDHeader          = "X-Request-ID"
	simulateTimeoutHeader    = "X-Debug-Simulate-Timeout"
	logErrorReasonNone       = "NONE"
	logErrorReasonTimeout    = "MODEL_TIMEOUT"
	logErrorReasonRequest    = "MODEL_REQUEST_FAILED"
	logErrorReasonEmptyBody  = "MODEL_EMPTY_RESPONSE"
	logErrorReasonTextMiss   = "MODEL_TEXT_NOT_FOUND"
	logErrorReasonRepairFail = "RESULT_JSON_REPAIR_FAILED"
	logErrorReasonBadMethod  = "METHOD_NOT_ALLOWED"
)

type analyzeRequestLog struct {
	RequestID     string
	DurationMS    int64
	ModelName     string
	ErrorReason   string
	Success       bool
	StatusCode    int
	Method        string
	Path          string
	BattleType    string
	LogTextLength int
}

func (h *Handler) Analyze(w http.ResponseWriter, r *http.Request) {
	requestID := fmt.Sprintf("analyze-%d", time.Now().UnixNano())
	startedAt := time.Now()
	statusCode := http.StatusOK
	errorReason := logErrorReasonNone
	success := false
	battleType := ""
	logTextLength := 0
	modelName := h.modelName()

	w.Header().Set(requestIDHeader, requestID)
	defer func() {
		logAnalyzeRequest(analyzeRequestLog{
			RequestID:     requestID,
			DurationMS:    time.Since(startedAt).Milliseconds(),
			ModelName:     modelName,
			ErrorReason:   errorReason,
			Success:       success,
			StatusCode:    statusCode,
			Method:        r.Method,
			Path:          r.URL.Path,
			BattleType:    battleType,
			LogTextLength: logTextLength,
		})
	}()

	if r.Method != http.MethodPost {
		statusCode = http.StatusMethodNotAllowed
		errorReason = logErrorReasonBadMethod
		w.WriteHeader(statusCode)
		return
	}

	log.Printf(
		"component=analyze_handler request_id=%s event=request_started method=%s path=%s remote_addr=%s",
		requestID,
		r.Method,
		r.URL.Path,
		r.RemoteAddr,
	)

	if h.analyzeService == nil {
		log.Printf("component=analyze_handler request_id=%s event=service_missing", requestID)
		statusCode = http.StatusInternalServerError
		errorReason = model.ErrCodeInternalError
		writeAppError(w, statusCode, model.AppError{
			Code:    model.ErrCodeInternalError,
			Message: "analyze service is not configured",
		})
		return
	}

	var req model.AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("component=analyze_handler request_id=%s event=decode_failed error=%q", requestID, err.Error())
		statusCode = http.StatusBadRequest
		errorReason = model.ErrCodeInvalidJSON
		writeAppError(w, statusCode, model.AppError{
			Code:    model.ErrCodeInvalidJSON,
			Message: "invalid analyze request json",
		})
		return
	}
	if appErr := req.NormalizeAndValidate(); appErr != nil {
		log.Printf("component=analyze_handler request_id=%s event=validate_failed code=%s error=%q", requestID, appErr.Code, appErr.Message)
		statusCode = http.StatusBadRequest
		errorReason = appErr.Code
		battleType = req.BattleType
		logTextLength = len(req.LogText)
		writeAppError(w, statusCode, *appErr)
		return
	}

	battleType = req.BattleType
	logTextLength = len(req.LogText)
	log.Printf(
		"component=analyze_handler request_id=%s event=request_decoded battle_type=%s build_tags_count=%d",
		requestID,
		req.BattleType,
		len(req.BuildTags),
	)

	ctx := llm.WithRequestID(r.Context(), requestID)
	if strings.TrimSpace(r.Header.Get(simulateTimeoutHeader)) == "1" {
		ctx = llm.WithSimulationMode(ctx, "timeout")
	}
	result, err := h.analyzeService.Analyze(ctx, req)
	if err != nil {
		statusCode, appErr, errorReasonValue := mapAnalyzeError(err)
		errorReason = errorReasonValue
		log.Printf("component=analyze_handler request_id=%s event=request_failed error=%q", requestID, err.Error())
		writeAppError(w, statusCode, appErr)
		return
	}

	success = true
	log.Printf(
		"component=analyze_handler request_id=%s event=request_succeeded duration_ms=%d raw_text_len=%d",
		requestID,
		time.Since(startedAt).Milliseconds(),
		len(result.RawText),
	)

	writeJSON(w, statusCode, map[string]any{
		"ok":   true,
		"data": result,
	})
}

func mapAnalyzeError(err error) (int, model.AppError, string) {
	var statusErr *llm.HTTPStatusError
	var appErr *model.AppError
	var netErr net.Error

	switch {
	case errors.As(err, &appErr):
		return http.StatusBadRequest, *appErr, appErr.Code
	case errors.Is(err, service.ErrModelJSONRepairFailed):
		return http.StatusBadGateway, model.AppError{
			Code:    model.ErrCodeInvalidModelJSON,
			Message: "model output is not valid JSON and could not be repaired",
		}, logErrorReasonRepairFail
	case errors.As(err, &statusErr):
		return http.StatusBadGateway, model.AppError{
			Code:    model.ErrCodeAnalyzeFailed,
			Message: statusErr.Error(),
		}, logErrorReasonRequest
	case errors.Is(err, llm.ErrEmptyPrompt):
		return http.StatusBadRequest, model.AppError{
			Code:    model.ErrCodeInvalidArgument,
			Message: err.Error(),
		}, model.ErrCodeInvalidArgument
	case errors.As(err, &netErr) && netErr.Timeout():
		return http.StatusBadGateway, model.AppError{
			Code:    model.ErrCodeAnalyzeFailed,
			Message: err.Error(),
		}, logErrorReasonTimeout
	case errors.Is(err, context.DeadlineExceeded):
		return http.StatusBadGateway, model.AppError{
			Code:    model.ErrCodeAnalyzeFailed,
			Message: err.Error(),
		}, logErrorReasonTimeout
	case errors.Is(err, llm.ErrEmptyResponse):
		return http.StatusBadGateway, model.AppError{
			Code:    model.ErrCodeAnalyzeFailed,
			Message: err.Error(),
		}, logErrorReasonEmptyBody
	case errors.Is(err, llm.ErrTextNotFound):
		return http.StatusBadGateway, model.AppError{
			Code:    model.ErrCodeAnalyzeFailed,
			Message: err.Error(),
		}, logErrorReasonTextMiss
	case errors.Is(err, service.ErrInvalidLLMJSON):
		return http.StatusBadGateway, model.AppError{
			Code:    model.ErrCodeInvalidModelJSON,
			Message: err.Error(),
		}, model.ErrCodeInvalidModelJSON
	case errors.Is(err, service.ErrInvalidAnalyzeResult):
		return http.StatusBadGateway, model.AppError{
			Code:    model.ErrCodeAnalyzeFailed,
			Message: err.Error(),
		}, model.ErrCodeAnalyzeFailed
	default:
		return http.StatusBadGateway, model.AppError{
			Code:    model.ErrCodeAnalyzeFailed,
			Message: err.Error(),
		}, model.ErrCodeAnalyzeFailed
	}
}

func logAnalyzeRequest(entry analyzeRequestLog) {
	modelName := entry.ModelName
	if modelName == "" {
		modelName = "unknown"
	}
	errorReason := entry.ErrorReason
	if errorReason == "" {
		errorReason = logErrorReasonNone
	}

	log.Printf(
		`component=analyze_request_log event=request_completed request_id=%s duration_ms=%d model_name=%q error_reason=%q success=%t status_code=%d method=%s path=%s battle_type=%q log_text_length=%d`,
		entry.RequestID,
		entry.DurationMS,
		modelName,
		errorReason,
		entry.Success,
		entry.StatusCode,
		entry.Method,
		entry.Path,
		entry.BattleType,
		entry.LogTextLength,
	)
}

func (h *Handler) modelName() string {
	if h == nil || h.analyzeService == nil {
		return "unknown"
	}
	modelName := h.analyzeService.ModelName()
	if modelName == "" {
		return "unknown"
	}
	return modelName
}
