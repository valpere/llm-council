package storage

import (
	"encoding/json"
	"os"
	"sync"
	"testing"
)

const testID = "12345678-1234-1234-1234-123456789abc"

func newTestStore(t *testing.T) *Store {
	t.Helper()
	return New(t.TempDir())
}

func TestCreate(t *testing.T) {
	s := newTestStore(t)
	conv, err := s.Create(testID)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if conv.ID != testID {
		t.Errorf("ID: got %q, want %q", conv.ID, testID)
	}
	if conv.Title != "New Conversation" {
		t.Errorf("Title: got %q, want %q", conv.Title, "New Conversation")
	}
	if conv.CreatedAt == "" {
		t.Error("CreatedAt is empty")
	}
	if len(conv.Messages) != 0 {
		t.Errorf("Messages: got %d, want 0", len(conv.Messages))
	}
	if _, err := os.Stat(s.path(testID)); err != nil {
		t.Errorf("JSON file not on disk: %v", err)
	}
}

func TestCreate_invalidID(t *testing.T) {
	s := newTestStore(t)
	_, err := s.Create("not-a-uuid")
	if err == nil {
		t.Error("expected error for invalid ID, got nil")
	}
}

func TestGet_missing(t *testing.T) {
	s := newTestStore(t)
	conv, err := s.Get(testID)
	if err != nil {
		t.Fatalf("Get on missing ID: %v", err)
	}
	if conv != nil {
		t.Errorf("expected nil for missing ID, got %+v", conv)
	}
}

func TestGet_roundTrip(t *testing.T) {
	s := newTestStore(t)
	created, err := s.Create(testID)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := s.Get(testID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil after Create")
	}
	if got.ID != created.ID {
		t.Errorf("ID: got %q, want %q", got.ID, created.ID)
	}
	if got.Title != created.Title {
		t.Errorf("Title: got %q, want %q", got.Title, created.Title)
	}
	if got.CreatedAt != created.CreatedAt {
		t.Errorf("CreatedAt: got %q, want %q", got.CreatedAt, created.CreatedAt)
	}
}

func TestAddMessage(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.Create(testID); err != nil {
		t.Fatalf("Create: %v", err)
	}

	msg := map[string]string{"role": "user", "content": "hello"}
	if err := s.AddMessage(testID, msg); err != nil {
		t.Fatalf("AddMessage: %v", err)
	}
	if err := s.AddMessage(testID, msg); err != nil {
		t.Fatalf("AddMessage (2nd): %v", err)
	}

	conv, err := s.Get(testID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(conv.Messages) != 2 {
		t.Errorf("MessageCount: got %d, want 2", len(conv.Messages))
	}
}

func TestUpdateTitle(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.Create(testID); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := s.UpdateTitle(testID, "My Title"); err != nil {
		t.Fatalf("UpdateTitle: %v", err)
	}

	conv, err := s.Get(testID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if conv.Title != "My Title" {
		t.Errorf("Title: got %q, want %q", conv.Title, "My Title")
	}
}

func TestList_sortedDescending(t *testing.T) {
	s := newTestStore(t)

	ids := []string{
		"aaaaaaaa-0000-0000-0000-000000000001",
		"aaaaaaaa-0000-0000-0000-000000000002",
		"aaaaaaaa-0000-0000-0000-000000000003",
	}
	timestamps := []string{
		"2024-01-01T10:00:00Z",
		"2024-01-03T10:00:00Z",
		"2024-01-02T10:00:00Z",
	}

	for i, id := range ids {
		conv := &Conversation{
			ID:        id,
			CreatedAt: timestamps[i],
			Title:     "conv-" + id,
			Messages:  []json.RawMessage{},
		}
		if err := s.save(conv); err != nil {
			t.Fatalf("save: %v", err)
		}
	}

	metas, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(metas) != 3 {
		t.Fatalf("List count: got %d, want 3", len(metas))
	}
	// Expect descending order: Jan 3, Jan 2, Jan 1
	if metas[0].CreatedAt != "2024-01-03T10:00:00Z" {
		t.Errorf("metas[0].CreatedAt: got %q, want 2024-01-03", metas[0].CreatedAt)
	}
	if metas[1].CreatedAt != "2024-01-02T10:00:00Z" {
		t.Errorf("metas[1].CreatedAt: got %q, want 2024-01-02", metas[1].CreatedAt)
	}
	if metas[2].CreatedAt != "2024-01-01T10:00:00Z" {
		t.Errorf("metas[2].CreatedAt: got %q, want 2024-01-01", metas[2].CreatedAt)
	}
}

func TestAddMessage_concurrent(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.Create(testID); err != nil {
		t.Fatalf("Create: %v", err)
	}

	const goroutines = 10
	const msgsPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := range goroutines {
		go func(n int) {
			defer wg.Done()
			msg := map[string]int{"goroutine": n}
			for range msgsPerGoroutine {
				if err := s.AddMessage(testID, msg); err != nil {
					t.Errorf("AddMessage goroutine %d: %v", n, err)
				}
			}
		}(i)
	}
	wg.Wait()

	conv, err := s.Get(testID)
	if err != nil {
		t.Fatalf("Get after concurrent writes: %v", err)
	}
	want := goroutines * msgsPerGoroutine
	if len(conv.Messages) != want {
		t.Errorf("final message count: got %d, want %d", len(conv.Messages), want)
	}
}
