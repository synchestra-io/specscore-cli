package cli

import (
	"encoding/json"
	"io"

	"gopkg.in/yaml.v3"
)

// yamlEnc is a minimal interface covering yaml.Encoder for testable error injection.
type yamlEnc interface {
	Encode(v any) error
	Close() error
}

// jsonEnc is a minimal interface covering json.Encoder for testable error injection.
type jsonEnc interface {
	Encode(v any) error
}

// Testable encoder factories. Tests replace these to inject failures.
var newYAMLEnc = func(w io.Writer) yamlEnc {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	return enc
}

var newJSONEnc = func(w io.Writer) jsonEnc {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc
}
