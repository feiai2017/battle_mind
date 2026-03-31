package model

// internal/model: 请求与响应结构定义。
type AnalyzeRequest struct {
	LogText string `json:"logText"`
}

type AnalyzeResult struct {
	Summary     string   `json:"summary"`
	Issues      []string `json:"issues"`
	Suggestions []string `json:"suggestions"`
	Confidence  float64  `json:"confidence"`
}
