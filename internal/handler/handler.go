package handler

import "github.com/feiai2017/battle_mind/internal/service"

// internal/handler: HTTP 接口层。
type Handler struct {
	analyzeService *service.AnalyzeService
}

func New(analyzeService ...*service.AnalyzeService) *Handler {
	var svc *service.AnalyzeService
	if len(analyzeService) > 0 {
		svc = analyzeService[0]
	}
	return &Handler{
		analyzeService: svc,
	}
}
