package chat

import (
	"io"
	"log/slog"
	"testing"

	"github.com/openinfer/openinfer-studio/internal/database"
	"github.com/openinfer/openinfer-studio/migrations"
)

type nullSink struct{}

func (nullSink) Publish(string, any) {}

func testService(t *testing.T) *Service {
	t.Helper()
	db, err := database.Open(t.TempDir(), migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	_ = slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewService(db.DB, nullSink{}, nil)
}

func TestConversationCRUD(t *testing.T) {
	s := testService(t)
	c, err := s.CreateConversation("model-1", "Test chat", "You are helpful.")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RenameConversation(c.ID, "Renamed"); err != nil {
		t.Fatal(err)
	}
	if err := s.ArchiveConversation(c.ID, true); err != nil {
		t.Fatal(err)
	}
	list, _ := s.ListConversations(false)
	if len(list) != 0 {
		t.Error("archived conversation should be hidden by default")
	}
	list, _ = s.ListConversations(true)
	if len(list) != 1 || list[0].Title != "Renamed" {
		t.Errorf("unexpected list: %+v", list)
	}
	if err := s.DeleteConversation(c.ID); err != nil {
		t.Fatal(err)
	}
	list, _ = s.ListConversations(true)
	if len(list) != 0 {
		t.Error("delete failed")
	}
}

// TestBranching verifies the message-tree behavior that powers edit and
// regenerate: two children of one parent form two branches.
func TestBranching(t *testing.T) {
	s := testService(t)
	c, _ := s.CreateConversation("m", "", "")

	u1, _ := s.addMessage(c.ID, "", "user", "hello")
	a1, _ := s.addMessage(c.ID, u1.ID, "assistant", "hi there")
	// Branch: regenerate from the same user message.
	a2, _ := s.addMessage(c.ID, u1.ID, "assistant", "alternative answer")
	// Continue the first branch.
	u2, _ := s.addMessage(c.ID, a1.ID, "user", "follow up")

	chain, err := s.chain(u2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(chain) != 3 || chain[1].ID != a1.ID {
		t.Fatalf("branch 1 chain wrong: %+v", chain)
	}

	chain2, err := s.chain(a2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(chain2) != 2 || chain2[1].ID != a2.ID {
		t.Fatalf("branch 2 chain wrong: %+v", chain2)
	}

	// Both branches must survive in the full message list.
	msgs, _ := s.Messages(c.ID)
	if len(msgs) != 4 {
		t.Errorf("want 4 messages, got %d", len(msgs))
	}
}

func TestLatestLeaf(t *testing.T) {
	s := testService(t)
	c, _ := s.CreateConversation("m", "", "")
	if leaf, _ := s.latestLeaf(c.ID); leaf != "" {
		t.Error("empty conversation must have no leaf")
	}
	u1, _ := s.addMessage(c.ID, "", "user", "one")
	a1, _ := s.addMessage(c.ID, u1.ID, "assistant", "two")
	leaf, err := s.latestLeaf(c.ID)
	if err != nil || leaf != a1.ID {
		t.Errorf("leaf = %q, want %q", leaf, a1.ID)
	}
}

func TestParamsMerge(t *testing.T) {
	body := map[string]any{"model": "x"}
	temp := 0.5
	maxTok := 100
	applyParams(body, GenParams{Temperature: &temp, MaxTokens: &maxTok, Stop: []string{"END"}})
	if body["temperature"] != 0.5 || body["max_tokens"] != 100 {
		t.Errorf("params not applied: %+v", body)
	}
	stops, ok := body["stop"].([]string)
	if !ok || stops[0] != "END" {
		t.Errorf("stop not applied: %+v", body["stop"])
	}
	if _, present := body["top_k"]; present {
		t.Error("unset params must not appear in the request")
	}
}

func TestJSONSchemaParam(t *testing.T) {
	body := map[string]any{}
	applyParams(body, GenParams{JSONSchema: `{"type":"object"}`})
	rf, ok := body["response_format"].(map[string]any)
	if !ok || rf["type"] != "json_schema" {
		t.Errorf("response_format malformed: %+v", body)
	}
}
