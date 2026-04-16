package main

import (
	"log/slog"
	"os"

	"github.com/joho/godotenv"
	"github.com/valpere/llm-council/internal/config"
)

func main() {
	// Load .env if present; ignore error so production environments without a
	// .env file work normally.
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration error", "error", err)
		os.Exit(1)
	}

	_ = cfg // wiring in L3.8
}
