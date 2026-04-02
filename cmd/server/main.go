package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/feiai2017/battle_mind/internal/config"
	"github.com/feiai2017/battle_mind/internal/handler"
	"github.com/feiai2017/battle_mind/internal/llm"
	"github.com/feiai2017/battle_mind/internal/service"
)

// cmd/server: 程序入口。
func main() {
	cfg, err := config.Load("config.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "startup failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf(
		"config loaded: port=%d, base_url=%s, model=%s, timeout_seconds=%d\n",
		cfg.Server.Port,
		cfg.Model.BaseURL,
		cfg.Model.Model,
		cfg.Model.TimeoutSeconds,
	)
	log.Printf(
		"component=server event=config_loaded port=%d base_url=%s model=%s timeout_seconds=%d",
		cfg.Server.Port,
		cfg.Model.BaseURL,
		cfg.Model.Model,
		cfg.Model.TimeoutSeconds,
	)

	llmClient, err := llm.NewClient(cfg.Model)
	if err != nil {
		fmt.Fprintf(os.Stderr, "startup failed: %v\n", err)
		os.Exit(1)
	}

	analyzeService := service.NewAnalyzeService(llmClient)
	h := handler.New(analyzeService)
	mux := http.NewServeMux()
	mux.HandleFunc("/health", h.Health)
	mux.HandleFunc("/analyze", h.Analyze)
	mux.HandleFunc("/tools/convert/analyze-request", h.ConvertAnalyzeRequest)
	log.Printf("component=server event=routes_registered routes=/health,/analyze,/tools/convert/analyze-request")

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	fmt.Printf("server starting on %s\n", addr)
	log.Printf("component=server event=listen_start addr=%s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Fprintf(os.Stderr, "server stopped: %v\n", err)
		log.Printf("component=server event=listen_stop addr=%s error=%q", addr, err.Error())
		os.Exit(1)
	}
}
