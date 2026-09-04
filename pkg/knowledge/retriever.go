package knowledge

import (
	"context"
	"sort"
)

type Retriever interface {
	Search(context.Context, string, int) ([]SearchResult, error)
}

type SemanticRetriever interface {
	Search(context.Context, string, int) ([]SearchResult, error)
}

type HybridRetriever struct {
	Local    Retriever
	Semantic SemanticRetriever
}

func (r HybridRetriever) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	local, err := r.Local.Search(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	if r.Semantic == nil {
		return local, nil
	}
	semantic, semanticErr := r.Semantic.Search(ctx, query, limit)
	if semanticErr != nil {
		return local, nil
	}
	byID := make(map[string]SearchResult, len(local)+len(semantic))
	for _, result := range local {
		byID[result.ID] = result
	}
	for _, result := range semantic {
		if existing, ok := byID[result.ID]; ok {
			existing.Source = "hybrid"
			existing.Score += result.Score
			byID[result.ID] = existing
			continue
		}
		byID[result.ID] = result
	}
	results := make([]SearchResult, 0, len(byID))
	for _, result := range byID {
		results = append(results, result)
	}
	sort.SliceStable(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}
