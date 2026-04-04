package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/feiai2017/battle_mind/internal/llm"
	"github.com/feiai2017/battle_mind/internal/model"
	"github.com/feiai2017/battle_mind/internal/service"
)

func (h *Handler) Analyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	requestID := fmt.Sprintf("analyze-%d", time.Now().UnixNano())
	startedAt := time.Now()
	log.Printf(
		"component=analyze_handler request_id=%s event=request_started method=%s path=%s remote_addr=%s",
		requestID,
		r.Method,
		r.URL.Path,
		r.RemoteAddr,
	)

	if h.analyzeService == nil {
		log.Printf("component=analyze_handler request_id=%s event=service_missing", requestID)
		writeAppError(w, http.StatusInternalServerError, model.AppError{
			Code:    model.ErrCodeInternalError,
			Message: "analyze service is not configured",
		})
		return
	}

	var req model.AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("component=analyze_handler request_id=%s event=decode_failed error=%q", requestID, err.Error())
		writeAppError(w, http.StatusBadRequest, model.AppError{
			Code:    model.ErrCodeInvalidJSON,
			Message: "invalid analyze request json",
		})
		return
	}
	if appErr := req.NormalizeAndValidate(); appErr != nil {
		log.Printf("component=analyze_handler request_id=%s event=validate_failed code=%s error=%q", requestID, appErr.Code, appErr.Message)
		writeAppError(w, http.StatusBadRequest, *appErr)
		return
	}
	log.Printf(
		"component=analyze_handler request_id=%s event=request_decoded battle_type=%s build_tags_count=%d",
		requestID,
		req.BattleType,
		len(req.BuildTags),
	)

	ctx := llm.WithRequestID(r.Context(), requestID)
	result, err := h.analyzeService.Analyze(ctx, req)
	if err != nil {
		writeAnalyzeError(w, requestID, err)
		return
	}
	log.Printf(
		"component=analyze_handler request_id=%s event=request_succeeded duration_ms=%d raw_text_len=%d",
		requestID,
		time.Since(startedAt).Milliseconds(),
		len(result.RawText),
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"data": result,
	})
}

func writeAnalyzeError(w http.ResponseWriter, requestID string, err error) {
	log.Printf("component=analyze_handler request_id=%s event=request_failed error=%q", requestID, err.Error())

	var statusErr *llm.HTTPStatusError
	var appErr *model.AppError
	switch {
	case errors.As(err, &appErr):
		writeAppError(w, http.StatusBadRequest, *appErr)
	case errors.As(err, &statusErr):
		writeAppError(w, http.StatusBadGateway, model.AppError{
			Code:    model.ErrCodeAnalyzeFailed,
			Message: statusErr.Error(),
		})
	case errors.Is(err, llm.ErrEmptyPrompt):
		writeAppError(w, http.StatusBadRequest, model.AppError{
			Code:    model.ErrCodeInvalidArgument,
			Message: err.Error(),
		})
	case errors.Is(err, llm.ErrEmptyResponse), errors.Is(err, llm.ErrTextNotFound):
		writeAppError(w, http.StatusBadGateway, model.AppError{
			Code:    model.ErrCodeAnalyzeFailed,
			Message: err.Error(),
		})
	case errors.Is(err, service.ErrInvalidLLMJSON), errors.Is(err, service.ErrInvalidAnalyzeResult):
		writeAppError(w, http.StatusBadGateway, model.AppError{
			Code:    model.ErrCodeAnalyzeFailed,
			Message: err.Error(),
		})
	default:
		writeAppError(w, http.StatusBadGateway, model.AppError{
			Code:    model.ErrCodeAnalyzeFailed,
			Message: err.Error(),
		})
	}
}
