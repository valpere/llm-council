package config

import (
	"errors"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// Config holds all server configuration sourced from environment variables.
// It contains raw primitive fields only — no domain types.
type Config struct {
	OpenRouterAPIKey            string
	DataDir                     string
	DefaultCouncilType          string
	Port                        string
	DefaultCouncilModels        []string
	DefaultCouncilChairmanModel string
	DefaultCouncilTemperature   float64
}

// Load reads configuration from environment variables and returns an error if
// any required variable is missing. It never panics.
func Load() (*Config, error) {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return nil, errors.New("OPENROUTER_API_KEY is required but not set")
	}

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "data/conversations"
	}

	councilType := os.Getenv("DEFAULT_COUNCIL_TYPE")
	if councilType == "" {
		councilType = "default"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8001"
	}

	var models []string
	if raw := os.Getenv("DEFAULT_COUNCIL_MODELS"); raw != "" {
		for _, m := range strings.Split(raw, ",") {
			if m = strings.TrimSpace(m); m != "" {
				models = append(models, m)
			}
		}
	}
	if len(models) == 0 {
		slog.Warn("DEFAULT_COUNCIL_MODELS not set; using local-dev fallback models")
		models = []string{
			"openai/gpt-4o-mini",
			"anthropic/claude-haiku-4-5",
			"google/gemini-flash-1.5",
		}
	}

	chairmanModel := os.Getenv("DEFAULT_COUNCIL_CHAIRMAN_MODEL")
	if chairmanModel == "" {
		slog.Warn("DEFAULT_COUNCIL_CHAIRMAN_MODEL not set; using local-dev fallback model")
		chairmanModel = "openai/gpt-4o-mini"
	}

	temperature := 0.7
	if raw := os.Getenv("DEFAULT_COUNCIL_TEMPERATURE"); raw != "" {
		if t, err := strconv.ParseFloat(raw, 64); err == nil {
			temperature = t
		}
	}

	return &Config{
		OpenRouterAPIKey:            apiKey,
		DataDir:                     dataDir,
		DefaultCouncilType:          councilType,
		Port:                        port,
		DefaultCouncilModels:        models,
		DefaultCouncilChairmanModel: chairmanModel,
		DefaultCouncilTemperature:   temperature,
	}, nil
}
