package council

import "sort"

// CalculateAggregateRankings computes aggregate rankings and Kendall's W coefficient
// from Stage 2 peer-review results.
//
// allLabels lists every anonymous label being ranked (e.g. "Response A").
// Judges with nil/empty Rankings receive midrank imputation: each item is assigned
// rank (n+1)/2 so they contribute without distorting the aggregate.
//
// Returns (nil, 0.0) when stage2 is empty, allLabels is empty, or all judges have
// nil Rankings.
//
// Kendall's W formula: W = 12·S / (k²·(n³−n))
// where S = Σ(Rj − R̄)², Rj = sum of ranks for item j, R̄ = k·(n+1)/2.
// W is clamped to [0.0, 1.0]; 1.0 = perfect agreement, 0.0 = no agreement.
func CalculateAggregateRankings(stage2 []StageTwoResult, allLabels []string) ([]RankedModel, float64) {
	n := len(allLabels)
	k := len(stage2)
	if n == 0 || k == 0 {
		return nil, 0.0
	}

	midrank := float64(n+1) / 2.0

	// rankSums[i] = sum of ranks assigned to allLabels[i] across all judges.
	rankSums := make([]float64, n)

	validJudges := 0
	for _, result := range stage2 {
		if len(result.Rankings) == 0 {
			// Midrank imputation: all items get (n+1)/2.
			for i := range rankSums {
				rankSums[i] += midrank
			}
			continue
		}
		validJudges++
		// Build rank-by-label map for this judge; unmentioned items get midrank.
		rankOf := make(map[string]float64, len(result.Rankings))
		for pos, label := range result.Rankings {
			rankOf[label] = float64(pos + 1)
		}
		for i, label := range allLabels {
			if r, ok := rankOf[label]; ok {
				rankSums[i] += r
			} else {
				rankSums[i] += midrank
			}
		}
	}

	if validJudges == 0 {
		return nil, 0.0
	}

	// Kendall's W.
	kf := float64(k)
	meanRankSum := kf * float64(n+1) / 2.0
	var S float64
	for _, rj := range rankSums {
		d := rj - meanRankSum
		S += d * d
	}
	denom := kf * kf * (float64(n)*float64(n)*float64(n) - float64(n))
	if denom == 0 {
		return nil, 0.0
	}
	W := 12.0 * S / denom
	if W < 0 {
		W = 0
	}
	if W > 1 {
		W = 1
	}

	// Build result sorted by average rank ascending (1.0 = always ranked first).
	ranked := make([]RankedModel, n)
	for i, label := range allLabels {
		ranked[i] = RankedModel{
			Model: label,
			Score: rankSums[i] / kf,
		}
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].Score < ranked[j].Score
	})
	return ranked, W
}
