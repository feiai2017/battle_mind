package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/feiai2017/battle_mind/internal/config"
	"github.com/feiai2017/battle_mind/internal/handler"
)

// cmd/server: 程序入口。
func main() {
	cfg, err := config.Load("config.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "startup failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf(
		"config loaded: port=%d, base_url=%s, timeout_seconds=%d\n",
		cfg.Server.Port,
		cfg.Model.BaseURL,
		cfg.Model.TimeoutSeconds,
	)

	h := handler.New()
	mux := http.NewServeMux()
	mux.HandleFunc("/health", h.Health)

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	fmt.Printf("server starting on %s\n", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Fprintf(os.Stderr, "server stopped: %v\n", err)
		os.Exit(1)
	}
}
