// Package event owns the shared event-dispatch plumbing for the `specscore`
// CLI: the Subscriber extension point, the Event envelope type, the envelope
// validator, the fan-out dispatcher, the built-in subscriber implementations
// (JsonlWriter, NoOp, Exec), and the events: config block loader.
//
// See `spec/features/cli/event/README.md` for the full Feature contract.
//
// This file currently scopes the package to the Subscriber interface and the
// Event/Actor/Artifact envelope types. Subscribers, validator, dispatcher, and
// config loader land in follow-on tasks.
package event
