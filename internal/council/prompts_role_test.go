package council

import (
	"strings"
	"testing"
)

func TestBuildRoleStage1Prompt_ContainsInstruction(t *testing.T) {
	role := Role{Name: "security", Instruction: "Find security vulnerabilities."}
	msgs := BuildRoleStage1Prompt(role, "Review this diff: +foo()")

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages (system + user), got %d", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Errorf("first message must be system, got %q", msgs[0].Role)
	}
	if !strings.Contains(msgs[0].Content, "Find security vulnerabilities.") {
		t.Errorf("system message must contain role instruction, got: %q", msgs[0].Content)
	}
	if msgs[1].Role != "user" {
		t.Errorf("second message must be user, got %q", msgs[1].Role)
	}
	if !strings.Contains(msgs[1].Content, "+foo()") {
		t.Errorf("user message must contain query, got: %q", msgs[1].Content)
	}
}

func TestBuildRoleChairmanPrompt_ContainsRoleNames(t *testing.T) {
	results := []StageOneResult{
		{Label: "security", Content: `[{"file":"main.go","line":10,"severity":"high","body":"SQL injection"}]`},
		{Label: "logic", Content: `[{"file":"main.go","line":20,"severity":"medium","body":"nil dereference"}]`},
	}
	msgs := BuildRoleChairmanPrompt("Review this diff", results)

	if len(msgs) == 0 {
		t.Fatal("expected at least one message")
	}
	combined := ""
	for _, m := range msgs {
		combined += m.Content
	}
	if !strings.Contains(combined, "security") {
		t.Error("chairman prompt must include role name 'security'")
	}
	if !strings.Contains(combined, "logic") {
		t.Error("chairman prompt must include role name 'logic'")
	}
	if !strings.Contains(combined, "SQL injection") {
		t.Error("chairman prompt must include findings content")
	}
	if !strings.Contains(combined, "Review this diff") {
		t.Error("chairman prompt must include original query")
	}
}

func TestBuildRoleChairmanPrompt_EmptyResults(t *testing.T) {
	msgs := BuildRoleChairmanPrompt("some query", nil)
	if len(msgs) == 0 {
		t.Fatal("must return messages even with empty results")
	}
}
