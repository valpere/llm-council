package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/valpere/llm-council/internal/council"
)

func TestSendReview_Returns200WithStage3Content(t *testing.T) {
	runner := &mockRunner{
		runFull: func(ctx context.Context, query, councilType string, onEvent council.EventFunc) error {
			if councilType != "code-review" {
				t.Errorf("expected council_type=code-review, got %q", councilType)
			}
			onEvent("stage1_complete", []council.StageOneResult{})
			onEvent("stage2_complete", council.Stage2CompleteData{})
			onEvent("stage3_complete", council.StageThreeResult{Content: "## Review\n\nLGTM"})
			return nil
		},
	}
	h := newTestHandler(okStorer(), runner)

	body := bytes.NewBufferString(`{"content": "diff --git a/main.go..."}`)
	req := httptest.NewRequest(http.MethodPost, "/api/conversations/"+testConvID+"/review", body)
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", testConvID)
	w := httptest.NewRecorder()
	h.sendReview(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var msg council.AssistantMessage
	if err := json.NewDecoder(w.Body).Decode(&msg); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if msg.Stage3.Content != "## Review\n\nLGTM" {
		t.Errorf("unexpected stage3 content: %q", msg.Stage3.Content)
	}
}

func TestSendReview_AlwaysUsesCodeReviewCouncilType(t *testing.T) {
	var capturedType string
	runner := &mockRunner{
		runFull: func(_ context.Context, _, councilType string, onEvent council.EventFunc) error {
			capturedType = councilType
			onEvent("stage3_complete", council.StageThreeResult{Content: "ok"})
			return nil
		},
	}
	h := newTestHandler(okStorer(), runner)

	req := httptest.NewRequest(http.MethodPost, "/api/conversations/"+testConvID+"/review",
		bytes.NewBufferString(`{"content":"diff"}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", testConvID)
	w := httptest.NewRecorder()
	h.sendReview(w, req)

	if capturedType != "code-review" {
		t.Errorf("expected council type 'code-review', got %q", capturedType)
	}
}

func TestSendReview_EmptyContent_Returns400(t *testing.T) {
	h := newTestHandler(okStorer(), &mockRunner{})
	req := httptest.NewRequest(http.MethodPost, "/api/conversations/"+testConvID+"/review",
		bytes.NewBufferString(`{"content":""}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", testConvID)
	w := httptest.NewRecorder()
	h.sendReview(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSendReview_InvalidUUID_Returns400(t *testing.T) {
	h := newTestHandler(okStorer(), &mockRunner{})
	req := httptest.NewRequest(http.MethodPost, "/api/conversations/not-a-uuid/review",
		bytes.NewBufferString(`{"content":"diff"}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "not-a-uuid")
	w := httptest.NewRecorder()
	h.sendReview(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSendReviewStream_EmitsSSEEvents(t *testing.T) {
	runner := &mockRunner{
		runFull: func(_ context.Context, _, _ string, onEvent council.EventFunc) error {
			onEvent("stage1_complete", []council.StageOneResult{})
			onEvent("stage2_complete", council.Stage2CompleteData{})
			onEvent("stage3_complete", council.StageThreeResult{Content: "final"})
			return nil
		},
	}
	h := newTestHandler(okStorer(), runner)

	req := httptest.NewRequest(http.MethodPost, "/api/conversations/"+testConvID+"/review/stream",
		bytes.NewBufferString(`{"content":"diff"}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", testConvID)
	w := httptest.NewRecorder()
	h.sendReviewStream(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{"stage1_complete", "stage2_complete", "stage3_complete", `"type":"complete"`} {
		if !strings.Contains(body, want) {
			t.Errorf("SSE body missing event %q", want)
		}
	}
}
