package handler

import (
	"context"

	"github.com/feiai2017/battle_mind/internal/model"
	"github.com/feiai2017/battle_mind/internal/output"
	"github.com/feiai2017/battle_mind/internal/service"
)

type AnalyzeService interface {
	Analyze(ctx context.Context, input model.AnalyzeInput) (output.AnalyzeOutput, error)
	ModelName() string
}

// internal/handler: HTTP 接口层。
type Handler struct {
	analyzeService AnalyzeService
}

func New(analyzeService ...AnalyzeService) *Handler {
	var svc AnalyzeService
	if len(analyzeService) > 0 {
		svc = analyzeService[0]
	}
	return &Handler{
		analyzeService: svc,
	}
}

var _ AnalyzeService = (*service.AnalyzeService)(nil)
