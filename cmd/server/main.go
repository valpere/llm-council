package main

import (
	"log/slog"
	"os"

	"github.com/valpere/llm-council/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration error", "error", err)
		os.Exit(1)
	}

	_ = cfg // wiring in L3.8
}
