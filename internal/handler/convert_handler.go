package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/feiai2017/battle_mind/internal/model"
	"github.com/feiai2017/battle_mind/internal/service"
)

func (h *Handler) ConvertAnalyzeRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var report model.BattleReport
	if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"ok":    false,
			"error": fmt.Sprintf("invalid battle report json: %v", err),
		})
		return
	}

	analyzeRequest := service.ConvertBattleReportToAnalyzeRequest(report)

	if r.URL.Query().Get("download") == "1" {
		filename := buildDownloadFilename(report.FloorID)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(analyzeRequest)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"data": analyzeRequest,
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func buildDownloadFilename(floorID string) string {
	name := strings.TrimSpace(floorID)
	if name == "" {
		name = "battle"
	}
	name = strings.ReplaceAll(name, " ", "_")
	return name + ".analyze_request.json"
}
