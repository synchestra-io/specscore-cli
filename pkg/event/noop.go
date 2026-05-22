package event

import "context"

// NoOp is the explicit-opt-out Subscriber: it accepts every event, performs no
// work, and returns nil. It exists so an operator can configure events: with a
// "noop" entry and signal "I have considered subscribers and chosen none"
// rather than relying on the absence of configuration. See AC:noop-discards in
// spec/features/cli/event/README.md.
type NoOp struct{}

// Deliver discards the event and returns nil. The implementation MUST NOT
// touch the filesystem, network, stdout, or stderr.
func (NoOp) Deliver(ctx context.Context, e Event) error { return nil }

// Name returns the literal identifier "noop".
func (NoOp) Name() string { return "noop" }
