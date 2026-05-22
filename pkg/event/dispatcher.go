package event

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
)

// dispatchStderr is the sink for per-subscriber failure log lines. It defaults
// to os.Stderr in production. Tests swap it for a *bytes.Buffer to assert the
// exact contracted format without touching the process's real stderr.
var dispatchStderr io.Writer = os.Stderr

// SubscriberFailure pairs a failing subscriber's Name() with the error its
// Deliver returned. The dispatcher emits one stderr line per entry; the verb
// layer additionally inspects the slice when mapping to exit codes.
type SubscriberFailure struct {
	Name string
	Err  error
}

// DispatchResult is the structured outcome of a Dispatch call. The verb layer
// maps it to the standard exit-code contract (REQ:dispatch-exit-codes):
//
//   - ValidationError != nil                                     -> exit 2
//   - Delivered > 0 OR len(subscribers) == 0                     -> exit 0
//   - Failed > 0 AND Delivered == 0 AND len(subscribers) > 0     -> exit 10
//
// Exit codes 3 (missing project root) and other 2-class failures (config
// validation) are produced by callers above Dispatch.
type DispatchResult struct {
	// ValidationError is non-nil when envelope validation failed; in that
	// case no subscriber was invoked.
	ValidationError error
	// Delivered counts subscribers whose Deliver returned nil.
	Delivered int
	// Failed counts subscribers whose Deliver returned a non-nil error.
	Failed int
	// Failures lists each non-nil Deliver error in declared order.
	Failures []SubscriberFailure
}

// Dispatch fans an envelope out to subscribers sequentially in declared order.
// It first validates the envelope; on validation failure it returns
// immediately with ValidationError populated and no subscriber invoked. On a
// valid envelope it calls each subscriber's Deliver; per-subscriber errors are
// logged to stderr in the contracted key=value form and the iteration
// continues to the next subscriber. Successful deliveries produce no stderr
// output (REQ:fan-out-dispatch).
//
// The failure line format is:
//
//	event-dispatch failure: subscriber=<Name()> event=<e.Name> error="<err.Error()>"
//
// Embedded double quotes and backslashes in err.Error() are escaped so the
// line round-trips through standard log parsers.
func Dispatch(ctx context.Context, e Event, subscribers []Subscriber) DispatchResult {
	if err := Validate(e); err != nil {
		return DispatchResult{ValidationError: err}
	}

	var res DispatchResult
	for _, sub := range subscribers {
		if err := sub.Deliver(ctx, e); err != nil {
			writeFailureLine(dispatchStderr, sub.Name(), e.Name, err)
			res.Failed++
			res.Failures = append(res.Failures, SubscriberFailure{Name: sub.Name(), Err: err})
			continue
		}
		res.Delivered++
	}
	return res
}

// writeFailureLine renders exactly one stderr failure line in the contracted
// format. The error message is escaped with quoteError so embedded double
// quotes and backslashes are preserved unambiguously.
func writeFailureLine(w io.Writer, subscriberName, eventName string, err error) {
	fmt.Fprintf(
		w,
		"event-dispatch failure: subscriber=%s event=%s error=\"%s\"\n",
		subscriberName,
		eventName,
		quoteError(err.Error()),
	)
}

// quoteError escapes backslashes first, then double quotes, so the result can
// be safely placed between literal double-quote delimiters. We deliberately
// do not use strconv.Quote because that adds outer quotes and would also
// escape non-ASCII bytes, which the contract does not require.
func quoteError(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}
