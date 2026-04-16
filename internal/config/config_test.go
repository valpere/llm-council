package config

import (
	"os"
	"testing"
)

// setenv sets an env var for the duration of the test and restores the prior
// value (or unsets it) via t.Cleanup.
func setenv(t *testing.T, key, value string) {
	t.Helper()
	prev, hadPrev := os.LookupEnv(key)
	os.Setenv(key, value)
	t.Cleanup(func() {
		if hadPrev {
			os.Setenv(key, prev)
		} else {
			os.Unsetenv(key)
		}
	})
}

// unsetenv unsets an env var for the duration of the test and restores the
// prior value via t.Cleanup.
func unsetenv(t *testing.T, key string) {
	t.Helper()
	prev, hadPrev := os.LookupEnv(key)
	os.Unsetenv(key)
	t.Cleanup(func() {
		if hadPrev {
			os.Setenv(key, prev)
		}
	})
}

// baseEnv sets the minimum required environment for config.Load() to succeed.
func baseEnv(t *testing.T) {
	t.Helper()
	setenv(t, "OPENROUTER_API_KEY", "sk-test")
}

// ── TestLoad_LLMBaseURL ────────────────────────────────────────────────────

func TestLoad_LLMBaseURL_Unset(t *testing.T) {
	baseEnv(t)
	unsetenv(t, "LLM_API_BASE_URL")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LLMBaseURL != "" {
		t.Errorf("LLMBaseURL: got %q, want %q", cfg.LLMBaseURL, "")
	}
}

func TestLoad_LLMBaseURL_ValidHTTPS(t *testing.T) {
	baseEnv(t)
	const target = "https://api.ollama.com/v1/chat/completions"
	setenv(t, "LLM_API_BASE_URL", target)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LLMBaseURL != target {
		t.Errorf("LLMBaseURL: got %q, want %q", cfg.LLMBaseURL, target)
	}
}

func TestLoad_LLMBaseURL_ValidHTTP(t *testing.T) {
	baseEnv(t)
	const target = "http://localhost:11434/v1/chat/completions"
	setenv(t, "LLM_API_BASE_URL", target)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LLMBaseURL != target {
		t.Errorf("LLMBaseURL: got %q, want %q", cfg.LLMBaseURL, target)
	}
}

func TestLoad_LLMBaseURL_InvalidScheme(t *testing.T) {
	baseEnv(t)
	setenv(t, "LLM_API_BASE_URL", "ftp://example.com/v1/chat/completions")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for ftp scheme, got nil")
	}
}

func TestLoad_LLMBaseURL_NotAURL(t *testing.T) {
	baseEnv(t)
	setenv(t, "LLM_API_BASE_URL", "not-a-url")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for non-URL value, got nil")
	}
}
