package property

// Test seams for Walk — allow tests to inject failures for Discover / Parse
// without preparing impossible filesystem state.
var (
	discoverFn = Discover
	parseFn    = Parse
)

// Walk combines Discover and Parse: it discovers every property file under
// `<specRoot>/features/` and invokes `fn` with the parsed Doc for each one,
// in slug-sorted order.
//
// Walk returns the first non-nil error from `fn` and stops iterating. I/O
// or parse errors abort with a wrapped error; callers that want to continue
// past a single bad file MUST handle the error inside `fn` and return nil.
// A missing `<specRoot>/features/` directory is a no-op.
func Walk(specRoot string, fn func(*Doc) error) error {
	discovered, err := discoverFn(specRoot)
	if err != nil {
		return err
	}
	for _, d := range discovered {
		doc, err := parseFn(d.Path)
		if err != nil {
			return err
		}
		if err := fn(doc); err != nil {
			return err
		}
	}
	return nil
}
