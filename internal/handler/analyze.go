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
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"ok":    false,
			"error": "analyze service is not configured",
		})
		return
	}

	var req model.AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("component=analyze_handler request_id=%s event=decode_failed error=%q", requestID, err.Error())
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"ok":    false,
			"error": fmt.Sprintf("invalid analyze request json: %v", err),
		})
		return
	}
	if err := req.NormalizeAndValidate(); err != nil {
		log.Printf("component=analyze_handler request_id=%s event=validate_failed error=%q", requestID, err.Error())
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"ok":    false,
			"error": fmt.Sprintf("invalid analyze request: %v", err),
		})
		return
	}
	log.Printf(
		"component=analyze_handler request_id=%s event=request_decoded battle_type=%s diagnosis_count=%d",
		requestID,
		req.Metadata.BattleType,
		len(req.Diagnosis),
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
	switch {
	case errors.As(err, &statusErr):
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"ok":    false,
			"error": statusErr.Error(),
		})
	case errors.Is(err, llm.ErrEmptyPrompt):
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"ok":    false,
			"error": err.Error(),
		})
	case errors.Is(err, llm.ErrEmptyResponse), errors.Is(err, llm.ErrTextNotFound):
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"ok":    false,
			"error": err.Error(),
		})
	case errors.Is(err, service.ErrInvalidLLMJSON), errors.Is(err, service.ErrInvalidAnalyzeResult):
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"ok":    false,
			"error": err.Error(),
		})
	default:
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"ok":    false,
			"error": err.Error(),
		})
	}
}
