package config

// internal/config: 配置读取与管理。
type Config struct {
	HTTPAddr   string
	LLMAPIKey  string
	LLMBaseURL string
}

// TODO(v0.1): 后续任务中从环境变量读取配置并设置默认值。
