// Package projectdef provides the specscore.yaml schema and read/write
// operations defined by the SpecScore Repo Config feature
// (https://specscore.md/repo-config).
package projectdef

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// SpecConfigFile is the canonical filename for the repo-level config,
// per the repo-config feature.
const SpecConfigFile = "specscore.yaml"

// SchemaHeader is the exact comment that MUST appear on line 1 of every
// specscore.yaml file. See repo-config#req:schema-header-comment.
const SchemaHeader = "# SpecScore Repo Config Schema: https://specscore.md/repo-config"

// Default values supplied when fields are omitted.
const (
	DefaultSpecsDirName = "specs"
	DefaultDocsDirName  = "docs"
	DefaultViewerName   = "SpecStudio"
	DefaultViewerURL    = "https://specstudio.synchestra.io/"
	DefaultModuleName   = "default"
)

// SpecConfig is the deserialized form of specscore.yaml.
type SpecConfig struct {
	Project      *ProjectConfig `yaml:"project,omitempty"`
	Projects     []string       `yaml:"projects,omitempty"`
	SpecsDirName string         `yaml:"specs_dir_name,omitempty"`
	DocsDirName  string         `yaml:"docs_dir_name,omitempty"`
	Viewer       *ViewerConfig  `yaml:"viewer,omitempty"`
	Modules      []ModuleConfig `yaml:"modules,omitempty"`
	Extras       map[string]any `yaml:",inline"`

	// viewerExplicitNull is set to true when YAML contains
	// `viewer: null` (or `~`, or an empty value) — the opt-out form
	// per repo-config#req:viewer-null-opts-out. It is NOT serialized;
	// callers should reconstruct via WithViewerSuppressed when writing.
	viewerExplicitNull bool
}

// ProjectConfig holds project identity. All fields are optional; when
// omitted, callers infer values from the working directory and git
// remote.
type ProjectConfig struct {
	Title        string         `yaml:"title,omitempty"`
	Host         string         `yaml:"host,omitempty"`
	Org          string         `yaml:"org,omitempty"`
	Repo         string         `yaml:"repo,omitempty"`
	Repositories []string       `yaml:"repositories,omitempty"`
	Extras       map[string]any `yaml:",inline"`
}

// ViewerConfig names the upstream viewer that renders SpecScore
// artifacts. Both fields are required when the block is present.
type ViewerConfig struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

// ModuleConfig is one entry in the `modules:` list — a code area inside
// the repo, optionally with its own specs/docs subdirectories.
type ModuleConfig struct {
	Name   string         `yaml:"name,omitempty"`
	Path   string         `yaml:"path,omitempty"`
	Code   []string       `yaml:"code,omitempty"`
	Extras map[string]any `yaml:",inline"`
}

// EffectiveSpecsDirName returns the configured specs dir name or the
// default.
func (c SpecConfig) EffectiveSpecsDirName() string {
	if c.SpecsDirName != "" {
		return c.SpecsDirName
	}
	return DefaultSpecsDirName
}

// EffectiveDocsDirName returns the configured docs dir name or the
// default.
func (c SpecConfig) EffectiveDocsDirName() string {
	if c.DocsDirName != "" {
		return c.DocsDirName
	}
	return DefaultDocsDirName
}

// EffectiveViewer reports the viewer to use for artifact links.
// suppressed is true when `viewer: null` was set explicitly — callers
// MUST omit any view link in that case (repo-config#req:viewer-null-opts-out).
// When the block is omitted entirely, name and url default to SpecStudio
// (repo-config#req:viewer-default-when-omitted).
func (c SpecConfig) EffectiveViewer() (name, url string, suppressed bool) {
	if c.viewerExplicitNull {
		return "", "", true
	}
	if c.Viewer != nil {
		return c.Viewer.Name, c.Viewer.URL, false
	}
	return DefaultViewerName, DefaultViewerURL, false
}

// EffectiveName returns the module's effective name per
// repo-config#req:module-name-deduction.
func (m ModuleConfig) EffectiveName() string {
	if m.Name != "" {
		return m.Name
	}
	if m.Path != "" {
		return filepath.Base(filepath.Clean(m.Path))
	}
	return DefaultModuleName
}

// EffectivePath returns the module's effective filesystem path,
// defaulting to "." (repo root) when no path is set.
func (m ModuleConfig) EffectivePath() string {
	if m.Path == "" {
		return "."
	}
	return filepath.Clean(m.Path)
}

// EffectiveModules returns the module list with the implicit-root
// default applied per repo-config#req:modules-default.
func (c SpecConfig) EffectiveModules() []ModuleConfig {
	if len(c.Modules) == 0 {
		return []ModuleConfig{{}}
	}
	return c.Modules
}

// Validate checks the structural invariants that don't require I/O:
// viewer mapping completeness and module path uniqueness/non-nesting.
// File-system checks (e.g. projects local-path resolution) belong to
// the linter.
func (c SpecConfig) Validate() error {
	if !c.viewerExplicitNull && c.Viewer != nil {
		if c.Viewer.Name == "" {
			return errors.New("viewer.name is required when the viewer block is a mapping (repo-config#req:viewer-explicit-values)")
		}
		if c.Viewer.URL == "" {
			return errors.New("viewer.url is required when the viewer block is a mapping (repo-config#req:viewer-explicit-values)")
		}
	}
	return validateModules(c.Modules)
}

// ValidateSchemaHeader verifies the first line of `data` is exactly the
// schema-header comment. Returns nil on success.
func ValidateSchemaHeader(data []byte) error {
	idx := bytes.IndexByte(data, '\n')
	var line []byte
	if idx < 0 {
		line = data
	} else {
		line = data[:idx]
	}
	line = bytes.TrimRight(line, "\r")
	if string(line) != SchemaHeader {
		return fmt.Errorf("schema header missing or malformed on line 1; expected %q (repo-config#req:schema-header-comment)", SchemaHeader)
	}
	return nil
}

// ReadSpecConfig reads dir/specscore.yaml and decodes it. The file must
// begin with the schema-header comment on line 1; otherwise an error
// is returned without further parsing.
func ReadSpecConfig(dir string) (SpecConfig, error) {
	var cfg SpecConfig
	path := filepath.Join(dir, SpecConfigFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("reading spec config: %w", err)
	}
	if err := ValidateSchemaHeader(data); err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing spec config: %w", err)
	}
	// Detect `viewer: null` vs viewer-omitted. yaml.v3 unmarshals both
	// to a nil *ViewerConfig, so we re-read at the node level to tell
	// them apart.
	cfg.viewerExplicitNull = detectViewerExplicitNull(data)
	return cfg, nil
}

// WriteSpecConfig serializes cfg to dir/specscore.yaml, prepending the
// schema-header comment as line 1.
func WriteSpecConfig(dir string, cfg SpecConfig) error {
	body, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshalling spec config: %w", err)
	}
	out := append([]byte(SchemaHeader+"\n\n"), body...)
	if err := os.WriteFile(filepath.Join(dir, SpecConfigFile), out, 0o644); err != nil {
		return fmt.Errorf("writing spec config: %w", err)
	}
	return nil
}

// IsViewerSuppressed reports whether viewer: null was set in the source.
func (c SpecConfig) IsViewerSuppressed() bool {
	return c.viewerExplicitNull
}

// validateModules enforces module-paths-unique and module-paths-non-nested
// from the repo-config feature.
func validateModules(modules []ModuleConfig) error {
	if len(modules) == 0 {
		return nil
	}
	// 1. Effective-path uniqueness — implicit root counts as ".".
	seen := make(map[string]int, len(modules))
	for i, m := range modules {
		p := m.EffectivePath()
		if prev, ok := seen[p]; ok {
			return fmt.Errorf(
				"duplicate module path %q at modules[%d] and modules[%d] (repo-config#req:module-paths-unique)",
				p, prev, i,
			)
		}
		seen[p] = i
	}
	// 2. Non-nesting — applies only to explicit paths. The implicit-root
	// module (no path:) is exempt per repo-config#req:module-paths-non-nested.
	type idxPath struct {
		i int
		p string
	}
	var explicit []idxPath
	for i, m := range modules {
		if m.Path == "" {
			continue
		}
		explicit = append(explicit, idxPath{i, filepath.Clean(m.Path)})
	}
	for i, a := range explicit {
		for j, b := range explicit {
			if i == j {
				continue
			}
			if isAncestorPath(a.p, b.p) {
				return fmt.Errorf(
					"module path %q (modules[%d]) is an ancestor of %q (modules[%d]) (repo-config#req:module-paths-non-nested)",
					a.p, a.i, b.p, b.i,
				)
			}
		}
	}
	return nil
}

// isAncestorPath reports whether `a` is a strict ancestor directory of `b`
// when both are interpreted as forward-slash paths.
func isAncestorPath(a, b string) bool {
	a = filepath.ToSlash(a)
	b = filepath.ToSlash(b)
	if a == b {
		return false
	}
	return strings.HasPrefix(b, a+"/")
}

// detectViewerExplicitNull parses the YAML at the node level to detect
// whether `viewer:` was explicitly null (vs omitted). Returns true only
// when the mapping has a "viewer" key and its value is a Null node or
// an empty alias to Null.
func detectViewerExplicitNull(data []byte) bool {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return false
	}
	// Document → MappingNode → key/value pairs
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return false
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		key := root.Content[i]
		val := root.Content[i+1]
		if key.Value != "viewer" {
			continue
		}
		// Scalar with empty value or YAML null tag = explicit null.
		if val.Kind == yaml.ScalarNode {
			if val.Tag == "!!null" || val.Value == "" || val.Value == "~" || val.Value == "null" || val.Value == "Null" || val.Value == "NULL" {
				return true
			}
		}
		return false
	}
	return false
}
