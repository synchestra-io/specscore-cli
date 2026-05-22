package event

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// stubSubscriber confirms the Subscriber interface shape — exactly two methods:
// Deliver(ctx, Event) error and Name() string. Any drift in either signature
// breaks compilation of this test, which is the AC's intent.
type stubSubscriber struct{ name string }

func (s stubSubscriber) Deliver(ctx context.Context, e Event) error { return nil }
func (s stubSubscriber) Name() string                               { return s.name }

func TestSubscriberInterfaceShape(t *testing.T) {
	var sub Subscriber = stubSubscriber{name: "stub"}
	if got := sub.Name(); got != "stub" {
		t.Fatalf("Name() = %q, want %q", got, "stub")
	}
	if err := sub.Deliver(context.Background(), Event{}); err != nil {
		t.Fatalf("Deliver returned error: %v", err)
	}
}

func TestEventStructFields(t *testing.T) {
	// Construct an Event literal with every named field populated. If any field
	// is renamed or removed, this will fail to compile.
	e := Event{
		Name:      "idea.drafted",
		Version:   1,
		UUID:      "00000000-0000-4000-8000-000000000000",
		Timestamp: time.Now().UTC(),
		Actor: Actor{
			Kind: "skill",
			ID:   "specstudio:ideate",
		},
		Artifact: Artifact{
			Type:     "idea",
			ID:       "demo",
			Path:     "spec/ideas/demo.md",
			Revision: "uncommitted",
		},
		Payload: json.RawMessage(`{}`),
	}
	if e.Name != "idea.drafted" {
		t.Fatalf("Name = %q", e.Name)
	}
	if e.Actor.Kind != "skill" {
		t.Fatalf("Actor.Kind = %q", e.Actor.Kind)
	}
	if e.Artifact.Path != "spec/ideas/demo.md" {
		t.Fatalf("Artifact.Path = %q", e.Artifact.Path)
	}
	if string(e.Payload) != "{}" {
		t.Fatalf("Payload = %q", string(e.Payload))
	}
}
