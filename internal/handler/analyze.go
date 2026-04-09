package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/feiai2017/battle_mind/internal/llm"
	"github.com/feiai2017/battle_mind/internal/logging"
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
	payloadKind := ""

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

	logging.LogFields("analyze_handler", "request_start", requestID, map[string]any{
		"method":      r.Method,
		"path":        r.URL.Path,
		"remote_addr": r.RemoteAddr,
	})

	if h.analyzeService == nil {
		statusCode = http.StatusInternalServerError
		errorReason = model.ErrCodeInternalError
		appErr := model.AppError{
			Code:    model.ErrCodeInternalError,
			Message: "analyze service is not configured",
		}
		logging.LogFields("analyze_handler", "request_fail", requestID, map[string]any{
			"code":    appErr.Code,
			"message": appErr.Message,
			"reason":  "service_missing",
		})
		writeLoggedAppError(w, statusCode, appErr, requestID)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		statusCode = http.StatusBadRequest
		errorReason = model.ErrCodeInvalidJSON
		appErr := model.AppError{
			Code:    model.ErrCodeInvalidJSON,
			Message: "invalid analyze request json",
		}
		logging.LogFields("analyze_handler", "request_fail", requestID, map[string]any{
			"code":    appErr.Code,
			"message": appErr.Message,
			"reason":  "read_body_failed",
			"error":   err.Error(),
		})
		writeLoggedAppError(w, statusCode, appErr, requestID)
		return
	}

	input, payloadKind, err := decodeAnalyzeInput(body)
	if err != nil {
		statusCode = http.StatusBadRequest
		errorReason = model.ErrCodeInvalidJSON
		appErr := model.AppError{
			Code:    model.ErrCodeInvalidJSON,
			Message: "invalid analyze request json",
		}
		logging.LogFields("analyze_handler", "request_fail", requestID, map[string]any{
			"code":    appErr.Code,
			"message": appErr.Message,
			"reason":  "decode_failed",
			"error":   err.Error(),
		})
		writeLoggedAppError(w, statusCode, appErr, requestID)
		return
	}
	if appErr := input.Request.NormalizeAndValidate(); appErr != nil {
		statusCode = http.StatusBadRequest
		errorReason = appErr.Code
		battleType = input.Request.BattleType
		logTextLength = len(input.Request.LogText)
		logging.LogFields("analyze_handler", "request_fail", requestID, map[string]any{
			"code":        appErr.Code,
			"message":     appErr.Message,
			"battle_type": battleType,
			"reason":      "validate_failed",
		})
		writeLoggedAppError(w, statusCode, *appErr, requestID)
		return
	}

	battleType = input.Request.BattleType
	logTextLength = len(input.Request.LogText)
	logging.LogFields("analyze_handler", "request_decoded", requestID, map[string]any{
		"payload_kind":     payloadKind,
		"battle_type":      input.Request.BattleType,
		"build_tags_count": len(input.Request.BuildTags),
		"log_text_length":  len(input.Request.LogText),
		"has_report":       input.Report != nil,
	})

	ctx := llm.WithRequestID(r.Context(), requestID)
	if strings.TrimSpace(r.Header.Get(simulateTimeoutHeader)) == "1" {
		ctx = llm.WithSimulationMode(ctx, "timeout")
	}
	result, err := h.analyzeService.Analyze(ctx, input)
	if err != nil {
		statusCode, appErr, errorReasonValue := mapAnalyzeError(err)
		errorReason = errorReasonValue
		logging.LogFields("analyze_handler", "request_fail", requestID, map[string]any{
			"status_code": statusCode,
			"error_code":  appErr.Code,
			"message":     appErr.Message,
			"reason":      errorReason,
			"error":       err.Error(),
		})
		writeLoggedAppError(w, statusCode, appErr, requestID)
		return
	}

	success = true
	logging.LogFields("analyze_handler", "request_success", requestID, map[string]any{
		"duration_ms":  time.Since(startedAt).Milliseconds(),
		"raw_text_len": len(result.RawText),
		"status_code":  statusCode,
	})

	responsePayload := map[string]any{
		"ok":   true,
		"data": result,
	}
	logging.LogJSONBlock("analyze_handler", "http_response_ok", requestID, responsePayload)
	writeJSON(w, statusCode, responsePayload)
}

func decodeAnalyzeInput(body []byte) (model.AnalyzeInput, string, error) {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return model.AnalyzeInput{}, "", io.EOF
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		return model.AnalyzeInput{}, "", err
	}

	if looksLikeBattleReport(envelope) {
		var report model.BattleReport
		if err := json.Unmarshal(body, &report); err != nil {
			return model.AnalyzeInput{}, "", err
		}
		return model.AnalyzeInput{
			Request: service.ConvertBattleReportToAnalyzeRequest(report),
			Report:  &report,
		}, "battle_report", nil
	}

	var req model.AnalyzeRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return model.AnalyzeInput{}, "", err
	}
	return model.AnalyzeInput{Request: req}, "analyze_request", nil
}

func looksLikeBattleReport(envelope map[string]json.RawMessage) bool {
	if len(envelope) == 0 {
		return false
	}
	battleReportKeys := []string{
		"events",
		"snapshots",
		"resultSummary",
		"aggregateMetrics",
		"floorContext",
		"buildContext",
		"reportVersion",
	}
	for _, key := range battleReportKeys {
		if _, ok := envelope[key]; ok {
			return true
		}
	}
	return false
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

	logging.LogFields("analyze_request", "request_completed", entry.RequestID, map[string]any{
		"battle_type":     entry.BattleType,
		"duration_ms":     entry.DurationMS,
		"error_reason":    errorReason,
		"log_text_length": entry.LogTextLength,
		"method":          entry.Method,
		"model_name":      modelName,
		"path":            entry.Path,
		"status_code":     entry.StatusCode,
		"success":         entry.Success,
	})
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

func writeLoggedAppError(w http.ResponseWriter, status int, appErr model.AppError, requestID string) {
	payload := ErrorResponse{Error: appErr}
	logging.LogJSONBlock("analyze_handler", "http_response_error", requestID, payload)
	writeJSON(w, status, payload)
}
