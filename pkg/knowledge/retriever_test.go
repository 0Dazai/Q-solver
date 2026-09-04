package knowledge

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

type failingSemanticRetriever struct{}

func (failingSemanticRetriever) Search(context.Context, string, int) ([]SearchResult, error) {
	return nil, errors.New("cloud unavailable")
}

type staticSemanticRetriever struct{ results []SearchResult }

func (s staticSemanticRetriever) Search(context.Context, string, int) ([]SearchResult, error) {
	return s.results, nil
}

func TestHybridRetrieverFallsBackToLocalWhenCloudFails(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "knowledge.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.IndexMarkdown(context.Background(), "resume.md", "# 项目\n\n实现实时音频采集和转写重连。"); err != nil {
		t.Fatal(err)
	}
	retriever := HybridRetriever{Local: store, Semantic: failingSemanticRetriever{}}
	results, err := retriever.Search(context.Background(), "实时音频", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Source != "local_fts" {
		t.Fatalf("expected local fallback result, got %+v", results)
	}
}

func TestHybridRetrieverDeduplicatesCloudAndLocalChunks(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "knowledge.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.IndexMarkdown(context.Background(), "resume.md", "# 项目\n\n实现实时音频采集。"); err != nil {
		t.Fatal(err)
	}
	local, err := store.Search(context.Background(), "实时音频", 5)
	if err != nil || len(local) != 1 {
		t.Fatalf("local fixture failed: %+v %v", local, err)
	}
	cloud := local[0]
	cloud.Source, cloud.Score = "cloud_vector", 0.95
	retriever := HybridRetriever{Local: store, Semantic: staticSemanticRetriever{results: []SearchResult{cloud}}}
	results, err := retriever.Search(context.Background(), "实时音频", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Source != "hybrid" {
		t.Fatalf("expected one fused result, got %+v", results)
	}
}
