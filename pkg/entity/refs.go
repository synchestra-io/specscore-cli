package entity

import (
	"path/filepath"
	"strings"
)

// ResolveRef takes a property item's `ref:` value (relative path or URL)
// and returns the absolute path of the referenced .property.md file,
// plus a boolean indicating whether the path resolved within specRoot.
//
// URLs return ("", false, nil) — the caller decides whether to flag the
// URL form as a broken reference. Today the URL form is permitted (per
// [entity#req:ref-target-exists] which allows the URL form "when
// cross-repo imports land"); lint MUST NOT treat it as a violation.
func ResolveRef(specRoot, entityPath, ref string) (string, bool, error) {
	return resolveRelativeOrURL(specRoot, entityPath, ref)
}

// ResolveInherits takes a frontmatter `inherits:` value and returns the
// absolute path of the referenced .entity.md file. Mirrors ResolveRef's
// URL handling so cross-repo inheritance and cross-repo property
// references share the same surface.
func ResolveInherits(specRoot, entityPath, inherits string) (string, bool, error) {
	return resolveRelativeOrURL(specRoot, entityPath, inherits)
}

// resolveRelativeOrURL is the shared implementation of ResolveRef and
// ResolveInherits. The two helpers exist as separate exports because
// callers consult them for different lint REQs and a future revision
// may diverge their semantics (e.g., when cross-repo property refs are
// permitted before cross-repo inherits).
func resolveRelativeOrURL(specRoot, entityPath, value string) (string, bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false, nil
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return "", false, nil
	}
	// Resolve relative to the entity file's directory.
	base := filepath.Dir(entityPath)
	resolved := filepath.Clean(filepath.Join(base, value))
	absResolved, err := filepath.Abs(resolved)
	if err != nil {
		return resolved, false, err
	}
	absRoot, err := filepath.Abs(specRoot)
	if err != nil {
		return absResolved, false, err
	}
	rel, relErr := filepath.Rel(absRoot, absResolved)
	if relErr != nil {
		return absResolved, false, nil
	}
	isLocal := !strings.HasPrefix(rel, "..")
	return absResolved, isLocal, nil
}
