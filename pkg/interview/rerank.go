package interview

import (
	"sort"
	"strings"

	"Q-Solver/pkg/knowledge"
)

const (
	rerankWideRecall    = 20
	rerankFinalLimit    = 3
	rerankBM25Weight    = 0.5
	rerankKeywordWeight = 0.3
	rerankTitleWeight   = 0.2
)

// rerankResult holds a search result alongside its per-component scores.
type rerankResult struct {
	result         knowledge.SearchResult
	bm25Normalized float64
	keywordOverlap float64
	titleMatch     float64
	combinedScore  float64
}

// FilterRelevantResults is the quality gate between recall and prompt
// injection. A local FTS result must contain at least one planned entity; this
// prevents an unrelated top-N set from entering the prompt merely because FTS
// returned something.
func FilterRelevantResults(results []knowledge.SearchResult, queryTerms []string) []knowledge.SearchResult {
	if len(results) == 0 || len(queryTerms) == 0 {
		return nil
	}
	filtered := make([]knowledge.SearchResult, 0, minInt(len(results), rerankFinalLimit))
	seenDocument := make(map[string]bool)
	for _, result := range results {
		text := strings.ToLower(result.TitlePath + "\n" + result.Content)
		matched := false
		for _, term := range queryTerms {
			term = strings.ToLower(strings.TrimSpace(term))
			if len([]rune(term)) >= 2 && strings.Contains(text, term) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		documentKey := result.DocumentID
		if documentKey == "" {
			documentKey = result.Path
		}
		if seenDocument[documentKey] {
			continue
		}
		seenDocument[documentKey] = true
		filtered = append(filtered, result)
		if len(filtered) >= rerankFinalLimit {
			break
		}
	}
	return filtered
}

// RerankResults performs wide-recall retrieval re-ranking.
// It takes up to 20 raw FTS5 results, normalizes BM25 scores, adds
// keyword-overlap and title-match components, and returns the top 6.
// The second return value is the combined normalized score for each result,
// intended for logging only.
func RerankResults(results []knowledge.SearchResult, query string, keywords ProfileKeywords) ([]knowledge.SearchResult, []float64) {
	if len(results) == 0 {
		return nil, nil
	}

	// 1. Normalize BM25 scores to [0, 1].
	minScore, maxScore := results[0].Score, results[0].Score
	for _, r := range results {
		if r.Score < minScore {
			minScore = r.Score
		}
		if r.Score > maxScore {
			maxScore = r.Score
		}
	}
	scoreRange := maxScore - minScore

	// 2. Compute per-component scores.
	reranked := make([]rerankResult, 0, len(results))
	for _, r := range results {
		bm25Norm := 0.5
		if scoreRange > 0 {
			bm25Norm = (r.Score - minScore) / scoreRange
		}

		keywordOverlap := computeKeywordOverlap(r, keywords)
		titleMatch := computeTitleMatch(r, query)

		combined := bm25Norm*rerankBM25Weight +
			keywordOverlap*rerankKeywordWeight +
			titleMatch*rerankTitleWeight

		reranked = append(reranked, rerankResult{
			result:         r,
			bm25Normalized: bm25Norm,
			keywordOverlap: keywordOverlap,
			titleMatch:     titleMatch,
			combinedScore:  combined,
		})
	}

	// 3. Sort by combined score (descending).
	sort.SliceStable(reranked, func(i, j int) bool {
		return reranked[i].combinedScore > reranked[j].combinedScore
	})

	// 4. Take top N.
	limit := rerankFinalLimit
	if len(reranked) < limit {
		limit = len(reranked)
	}
	topResults := make([]knowledge.SearchResult, limit)
	normalizedScores := make([]float64, limit)
	for i := 0; i < limit; i++ {
		topResults[i] = reranked[i].result
		normalizedScores[i] = reranked[i].combinedScore
	}

	return topResults, normalizedScores
}

// computeKeywordOverlap measures how many profile keywords appear in the
// result content and title, normalized by the total keyword count.
func computeKeywordOverlap(r knowledge.SearchResult, keywords ProfileKeywords) float64 {
	if len(keywords.Terms) == 0 {
		return 0
	}
	text := r.Content + " " + r.TitlePath
	matches := keywords.Matches(text)
	overlap := float64(len(matches)) / float64(len(keywords.Terms))
	if overlap > 1.0 {
		return 1.0
	}
	return overlap
}

// computeTitleMatch measures how many query segments appear in the result
// title path, normalized by the number of valid query segments.
func computeTitleMatch(r knowledge.SearchResult, query string) float64 {
	segments := splitProfileSegments(query)
	if len(segments) == 0 {
		return 0
	}
	titleLower := strings.ToLower(r.TitlePath)
	matchCount := 0
	validSegments := 0
	for _, seg := range segments {
		if len([]rune(seg)) < 2 {
			continue
		}
		validSegments++
		if strings.Contains(titleLower, strings.ToLower(seg)) {
			matchCount++
		}
	}
	if validSegments == 0 {
		return 0
	}
	match := float64(matchCount) / float64(validSegments)
	if match > 1.0 {
		return 1.0
	}
	return match
}
