package interviewhistory

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestStorePersistsThreeRoleTimeline(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "interviews.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	session := Session{ID: "s1", StartedAt: time.Unix(10, 0)}
	if err := store.StartSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	for _, message := range []Message{
		{ID: "q1", SessionID: "s1", TurnID: "t1", Role: RoleInterviewer, Content: "请介绍项目", Status: StatusFinal},
		{ID: "a1", SessionID: "s1", TurnID: "t1", Role: RoleAISuggestion, Content: "我主要负责……", Status: StatusFinal},
		{ID: "c1", SessionID: "s1", TurnID: "t1", Role: RoleCandidate, Content: "我当时负责……", Status: StatusFinal},
	} {
		if err := store.UpsertMessage(ctx, message); err != nil {
			t.Fatal(err)
		}
	}
	messages, err := store.ListMessages(ctx, "s1")
	if err != nil || len(messages) != 3 {
		t.Fatalf("timeline=%+v err=%v", messages, err)
	}
	if messages[0].Role != RoleInterviewer || messages[2].Role != RoleCandidate {
		t.Fatalf("unexpected order: %+v", messages)
	}
}

func TestStoreReturnsEmptySlices(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "interviews.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	sessions, err := store.ListSessions(context.Background(), 20)
	if err != nil || sessions == nil {
		t.Fatalf("sessions=%+v err=%v", sessions, err)
	}
	messages, err := store.ListMessages(context.Background(), "missing")
	if err != nil || messages == nil {
		t.Fatalf("messages=%+v err=%v", messages, err)
	}
}
