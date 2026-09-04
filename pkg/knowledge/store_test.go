package knowledge

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestQuoteFTSQueryDoesNotCreateCrossTermTrigrams(t *testing.T) {
	query := quoteFTSQuery("Java 智慧课堂")
	if strings.Contains(query, `"va智"`) || strings.Contains(query, `"a智慧"`) {
		t.Fatalf("query contains cross-term trigrams: %s", query)
	}
	for _, expected := range []string{`"Jav"`, `"智慧课"`} {
		if !strings.Contains(query, expected) {
			t.Fatalf("query missing %s: %s", expected, query)
		}
	}
}

func TestStoreIndexesSearchesUpdatesAndDeletesMarkdown(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(filepath.Join(t.TempDir(), "knowledge.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.IndexMarkdown(ctx, "resume.md", "# 项目\n\n负责实时音频链路和 ASR 重连。"); err != nil {
		t.Fatal(err)
	}
	documents, err := store.ListDocuments(ctx)
	if err != nil || len(documents) != 1 || documents[0].Path != "resume.md" {
		t.Fatalf("indexed document inventory mismatch: %+v %v", documents, err)
	}
	results, err := store.Search(ctx, "实时音频重连", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Path != "resume.md" {
		t.Fatalf("unexpected search result: %+v", results)
	}
	pinned, err := store.PinnedMarkdownContext(ctx, 2000)
	if err != nil || len(pinned) != 1 || !strings.Contains(pinned[0], "ASR 重连") {
		t.Fatalf("unexpected pinned Markdown: %+v %v", pinned, err)
	}

	if err := store.IndexMarkdown(ctx, "resume.md", "# 项目\n\n负责本地资料检索。"); err != nil {
		t.Fatal(err)
	}
	oldResults, err := store.Search(ctx, "实时音频", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(oldResults) != 0 {
		t.Fatalf("stale chunks must be replaced: %+v", oldResults)
	}

	if err := store.DeleteDocument(ctx, "resume.md"); err != nil {
		t.Fatal(err)
	}
	results, err = store.Search(ctx, "资料检索", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("deleted document is still searchable: %+v", results)
	}
}

func TestStoreReturnsEmptyCollectionsInsteadOfNil(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(filepath.Join(t.TempDir(), "knowledge.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	documents, err := store.ListDocuments(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if documents == nil {
		t.Fatal("empty document inventory must be encoded as [] instead of null")
	}

	results, err := store.Search(ctx, "", 5)
	if err != nil {
		t.Fatal(err)
	}
	if results == nil {
		t.Fatal("empty search results must be encoded as [] instead of null")
	}
}
