package council

import (
	"context"
	"time"

	"llm-council/internal/openrouter"
)

// LLMClient is the interface for querying language models, allowing Council
// to be tested without a real OpenRouter connection.
type LLMClient interface {
	QueryModel(ctx context.Context, model string, messages []openrouter.Message, timeout time.Duration) (*openrouter.Response, error)
	QueryModelsParallel(ctx context.Context, models []string, messages []openrouter.Message, timeout time.Duration) []openrouter.ModelResult
}

// Runner executes the 3-stage LLM council deliberation pipeline.
// Implementations must be safe for concurrent use.
type Runner interface {
	Stage1CollectResponses(ctx context.Context, query string) ([]StageOneResult, error)
	Stage2CollectRankings(ctx context.Context, query string, stage1 []StageOneResult) ([]StageTwoResult, map[string]string, error)
	Stage3SynthesizeFinal(ctx context.Context, query string, stage1 []StageOneResult, stage2 []StageTwoResult) (StageThreeResult, error)
	GenerateTitle(ctx context.Context, query string) string
	RunFull(ctx context.Context, query string) (Result, error)
	CalculateAggregateRankings(stage2 []StageTwoResult, labelToModel map[string]string) []AggregateRanking
}

// Compile-time interface satisfaction checks.
var _ Runner = (*Council)(nil)
var _ LLMClient = (*openrouter.Client)(nil)
