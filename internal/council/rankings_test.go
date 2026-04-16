package council

import (
	"math"
	"testing"
)

func TestCalculateAggregateRankings_FullAgreement(t *testing.T) {
	labels := []string{"Response A", "Response B", "Response C"}
	stage2 := []StageTwoResult{
		{Rankings: []string{"Response A", "Response B", "Response C"}},
		{Rankings: []string{"Response A", "Response B", "Response C"}},
	}
	ranked, W := CalculateAggregateRankings(stage2, labels)
	if math.Abs(W-1.0) > 1e-9 {
		t.Errorf("W: got %.6f, want 1.0 (full agreement)", W)
	}
	if len(ranked) != 3 {
		t.Fatalf("len(ranked): got %d, want 3", len(ranked))
	}
	if ranked[0].Model != "Response A" {
		t.Errorf("top-ranked: got %q, want %q", ranked[0].Model, "Response A")
	}
}

func TestCalculateAggregateRankings_NoAgreement(t *testing.T) {
	// Two judges with reversed rankings: rank sums are equal → W = 0.
	labels := []string{"Response A", "Response B", "Response C"}
	stage2 := []StageTwoResult{
		{Rankings: []string{"Response A", "Response B", "Response C"}},
		{Rankings: []string{"Response C", "Response B", "Response A"}},
	}
	_, W := CalculateAggregateRankings(stage2, labels)
	if math.Abs(W) > 1e-9 {
		t.Errorf("W: got %.6f, want 0.0 (no agreement)", W)
	}
}

func TestCalculateAggregateRankings_OneMissingMidrank(t *testing.T) {
	// Judge 2 has nil Rankings → midrank imputation (rank 2.0 for all 3 items).
	// Judge 1: A=1, B=2, C=3
	// Judge 2: A=2, B=2, C=2 (midrank)
	// Rank sums: A=3, B=4, C=5  mean=4  S=1+0+1=2  W=12*2/(4*24)=0.25
	labels := []string{"Response A", "Response B", "Response C"}
	stage2 := []StageTwoResult{
		{Rankings: []string{"Response A", "Response B", "Response C"}},
		{Rankings: nil},
	}
	ranked, W := CalculateAggregateRankings(stage2, labels)
	wantW := 0.25
	if math.Abs(W-wantW) > 1e-9 {
		t.Errorf("W: got %.6f, want %.6f (one missing → midrank)", W, wantW)
	}
	if len(ranked) == 0 || ranked[0].Model != "Response A" {
		t.Errorf("top-ranked: got %v, want Response A", ranked)
	}
}

func TestCalculateAggregateRankings_AllMissing(t *testing.T) {
	labels := []string{"Response A", "Response B"}
	stage2 := []StageTwoResult{
		{Rankings: nil},
		{Rankings: nil},
	}
	ranked, W := CalculateAggregateRankings(stage2, labels)
	if ranked != nil {
		t.Errorf("ranked: got %v, want nil", ranked)
	}
	if W != 0.0 {
		t.Errorf("W: got %v, want 0.0", W)
	}
}

func TestCalculateAggregateRankings_Empty(t *testing.T) {
	ranked, W := CalculateAggregateRankings(nil, []string{"Response A"})
	if ranked != nil || W != 0.0 {
		t.Errorf("empty stage2: got ranked=%v W=%v, want nil 0.0", ranked, W)
	}
	ranked, W = CalculateAggregateRankings([]StageTwoResult{{Rankings: []string{"Response A"}}}, nil)
	if ranked != nil || W != 0.0 {
		t.Errorf("empty labels: got ranked=%v W=%v, want nil 0.0", ranked, W)
	}
}

func TestCalculateAggregateRankings_SingleJudge(t *testing.T) {
	// A single complete ranking always produces W = 1.0.
	labels := []string{"Response A", "Response B", "Response C"}
	stage2 := []StageTwoResult{
		{Rankings: []string{"Response B", "Response A", "Response C"}},
	}
	ranked, W := CalculateAggregateRankings(stage2, labels)
	if math.Abs(W-1.0) > 1e-9 {
		t.Errorf("W: got %.6f, want 1.0 (single judge)", W)
	}
	if len(ranked) == 0 || ranked[0].Model != "Response B" {
		t.Errorf("top-ranked: got %v, want Response B", ranked)
	}
}

func TestCalculateAggregateRankings_TwoJudgeManual(t *testing.T) {
	// Judge 1: A(1) B(2) C(3)   Judge 2: A(2) B(1) C(3)
	// Rank sums: A=3, B=3, C=6   mean=4   S=1+1+4=6
	// W = 12*6 / (4*24) = 72/96 = 0.75
	labels := []string{"Response A", "Response B", "Response C"}
	stage2 := []StageTwoResult{
		{Rankings: []string{"Response A", "Response B", "Response C"}},
		{Rankings: []string{"Response B", "Response A", "Response C"}},
	}
	ranked, W := CalculateAggregateRankings(stage2, labels)
	wantW := 0.75
	if math.Abs(W-wantW) > 1e-9 {
		t.Errorf("W: got %.6f, want %.6f", W, wantW)
	}
	// C should be bottom-ranked (score 3.0).
	if len(ranked) < 3 || ranked[2].Model != "Response C" {
		t.Errorf("bottom-ranked: got %v, want Response C", ranked)
	}
}
