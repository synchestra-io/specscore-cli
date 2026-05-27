package entity

// Test seams for Walk — allow tests to inject failures for Discover / Parse
// without preparing impossible filesystem state.
var (
	discoverFn = Discover
	parseFn    = Parse
)

// Walk is a convenience wrapper: Discover + Parse, invoking fn for each
// discovered entity. Returns the first non-nil error from fn; nil
// otherwise. I/O errors during discovery are returned as-is. Parse
// errors are propagated only when Parse itself returns one (I/O
// failure); malformed-but-readable files are still passed to fn with a
// partial Doc, matching Parse's resilience contract.
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
