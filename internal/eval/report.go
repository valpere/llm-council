package eval

import (
	"encoding/json"
	"fmt"
	"io"
)

// Summary aggregates per-case verdicts into the four-bucket counter that
// shows up on stdout at the end of a run.
type Summary struct {
	CouncilWon  int
	BaselineWon int
	Ties        int
	Errors      int
}

// Aggregate counts verdicts across results. Unknown verdicts (which should
// only appear if a future Verdict constant is added without updating this
// function) fall through to the Errors bucket.
func Aggregate(results []Result) Summary {
	var s Summary
	for _, r := range results {
		switch r.JudgeVerdict {
		case VerdictCouncil:
			s.CouncilWon++
		case VerdictBaseline:
			s.BaselineWon++
		case VerdictTie:
			s.Ties++
		default:
			s.Errors++
		}
	}
	return s
}

// Format renders the summary as a single human-readable line for stdout.
// `total` is passed in (rather than reconstructed from the buckets) so the
// caller can use len(results) directly without arithmetic.
func (s Summary) Format(total int) string {
	return fmt.Sprintf(
		"council won %d/%d · tied %d/%d · baseline won %d/%d · errors %d/%d",
		s.CouncilWon, total,
		s.Ties, total,
		s.BaselineWon, total,
		s.Errors, total,
	)
}

// WriteOutput serialises the {meta, results} envelope as indented JSON.
// Indentation matters: a human will read this file when investigating a
// regression.
func WriteOutput(w io.Writer, meta Meta, results []Result) error {
	out := Output{Meta: meta, Results: results}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return fmt.Errorf("encode output: %w", err)
	}
	return nil
}
