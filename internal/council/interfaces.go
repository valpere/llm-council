package council

import "context"

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
