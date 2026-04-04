package handler

import (
	"net/http"

	"github.com/feiai2017/battle_mind/internal/model"
)

type ErrorResponse struct {
	Error model.AppError `json:"error"`
}

func writeAppError(w http.ResponseWriter, status int, appErr model.AppError) {
	writeJSON(w, status, ErrorResponse{
		Error: appErr,
	})
}
