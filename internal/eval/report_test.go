package eval

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestAggregate_MixedVerdicts(t *testing.T) {
	results := []Result{
		{JudgeVerdict: VerdictCouncil},
		{JudgeVerdict: VerdictCouncil},
		{JudgeVerdict: VerdictCouncil},
		{JudgeVerdict: VerdictCouncil},
		{JudgeVerdict: VerdictCouncil},
		{JudgeVerdict: VerdictBaseline},
		{JudgeVerdict: VerdictBaseline},
		{JudgeVerdict: VerdictTie},
		{JudgeVerdict: VerdictTie},
		{JudgeVerdict: VerdictTie},
		{JudgeVerdict: VerdictError},
		{JudgeVerdict: VerdictError},
	}
	got := Aggregate(results)
	want := Summary{CouncilWon: 5, BaselineWon: 2, Ties: 3, Errors: 2}
	if got != want {
		t.Errorf("Aggregate: got %+v, want %+v", got, want)
	}
}

func TestAggregate_EmptyInput(t *testing.T) {
	got := Aggregate(nil)
	want := Summary{}
	if got != want {
		t.Errorf("Aggregate(nil): got %+v, want zero", got)
	}
}

func TestAggregate_UnknownVerdictCountsAsError(t *testing.T) {
	results := []Result{{JudgeVerdict: Verdict("???")}}
	got := Aggregate(results)
	if got.Errors != 1 {
		t.Errorf("unknown verdict should count as error, got Errors=%d", got.Errors)
	}
}

func TestSummary_Format(t *testing.T) {
	s := Summary{CouncilWon: 5, BaselineWon: 2, Ties: 3, Errors: 1}
	got := s.Format(11)
	want := "council won 5/11 · tied 3/11 · baseline won 2/11 · errors 1/11"
	if got != want {
		t.Errorf("Format: got %q, want %q", got, want)
	}
}

func TestSummary_Format_ZeroTotal(t *testing.T) {
	s := Summary{}
	got := s.Format(0)
	if !strings.Contains(got, "0/0") {
		t.Errorf("Format(0) should still render counters, got %q", got)
	}
}

func TestWriteOutput_RoundTripsThroughJSON(t *testing.T) {
	meta := Meta{
		Seed:          42,
		InputSHA256:   "abcd",
		BaselineModel: "openai/gpt-4o-mini",
		JudgeModel:    "anthropic/claude-haiku-4-5",
		CouncilType:   "default",
	}
	results := []Result{
		{
			ID:                 "case-1",
			Prompt:             "What is foo?",
			CouncilAnswer:      "council answer",
			BaselineAnswer:     "baseline answer",
			CouncilModel:       "default:synthesis",
			BaselineModel:      "openai/gpt-4o-mini",
			JudgeVerdict:       VerdictCouncil,
			JudgeExplanation:   "more focused",
			CouncilConsensusW:  0.73,
			CouncilDurationMs:  14820,
			BaselineDurationMs: 1200,
		},
	}

	var buf bytes.Buffer
	if err := WriteOutput(&buf, meta, results); err != nil {
		t.Fatalf("WriteOutput: %v", err)
	}

	// Must be parseable as Output.
	var got Output
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v\nbody: %s", err, buf.String())
	}
	if got.Meta != meta {
		t.Errorf("meta round-trip: got %+v, want %+v", got.Meta, meta)
	}
	if len(got.Results) != 1 {
		t.Fatalf("results round-trip: got %d, want 1", len(got.Results))
	}
	if got.Results[0] != results[0] {
		t.Errorf("result round-trip: got %+v, want %+v", got.Results[0], results[0])
	}

	// Pretty-printed body keeps it diff-able for humans investigating a
	// regression. Pin one structural property: indented arrays.
	if !strings.Contains(buf.String(), "  \"meta\": {") {
		t.Errorf("output should be indented for readability, got: %s", buf.String())
	}
}

func TestWriteOutput_OmitsErrorFieldWhenEmpty(t *testing.T) {
	// Per the json:"error,omitempty" tag, a successful result must not write
	// an empty "error" field — that would visually pollute every successful
	// row in a results file.
	results := []Result{{ID: "x", JudgeVerdict: VerdictCouncil}}
	var buf bytes.Buffer
	if err := WriteOutput(&buf, Meta{}, results); err != nil {
		t.Fatalf("WriteOutput: %v", err)
	}
	if strings.Contains(buf.String(), `"error"`) {
		t.Errorf("empty error field should be omitted, got: %s", buf.String())
	}
}

func TestWriteOutput_IncludesErrorFieldWhenSet(t *testing.T) {
	results := []Result{{ID: "x", JudgeVerdict: VerdictError, Error: "council failed"}}
	var buf bytes.Buffer
	if err := WriteOutput(&buf, Meta{}, results); err != nil {
		t.Fatalf("WriteOutput: %v", err)
	}
	if !strings.Contains(buf.String(), `"error": "council failed"`) {
		t.Errorf("non-empty error field should appear, got: %s", buf.String())
	}
}
