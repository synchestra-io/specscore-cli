package issue

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Parse reads the file at path and returns a parsed Issue. Parse is
// resilient: it returns a partial Issue even when the file is malformed
// (no frontmatter, invalid YAML, missing keys). Callers (lint rules)
// decide what is an error.
//
// Returns (nil, err) only when the file cannot be read.
func Parse(path string) (*Issue, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseBytes(path, content), nil
}

func parseBytes(path string, content []byte) *Issue {
	iss := &Issue{
		Path:        path,
		Slug:        strings.TrimSuffix(filepath.Base(path), ".md"),
		Frontmatter: map[string]string{},
	}
	front, body, ok := splitFrontmatter(string(content))
	if !ok {
		return iss
	}
	iss.HasFrontmatter = true
	iss.Body = body

	keys, order, err := parseFrontmatterKeys(front)
	if err != nil {
		return iss
	}
	iss.Frontmatter = keys
	iss.FrontmatterKeyOrder = order
	iss.Type = keys["type"]
	return iss
}

// splitFrontmatter mirrors pkg/lint.splitFrontmatter — extracts the
// leading YAML frontmatter block delimited by `---` lines. Duplicated
// here (rather than imported) to avoid a circular dep: pkg/lint already
// imports artifact packages, not the other way around.
func splitFrontmatter(content string) (front, body string, ok bool) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimRight(lines[0], "\r") != "---" {
		return "", "", false
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], "\r") == "---" {
			front = strings.Join(lines[1:i], "\n")
			body = strings.Join(lines[i+1:], "\n")
			return front, body, true
		}
	}
	return "", "", false
}

// parseFrontmatterKeys parses a top-level YAML mapping and returns the
// key/value pairs (values stringified) plus the order in which keys
// appeared. An empty frontmatter is treated as a valid empty mapping.
// Mirrors pkg/lint.parseFrontmatterKeys (see splitFrontmatter comment
// for the rationale on duplication).
func parseFrontmatterKeys(front string) (map[string]string, []string, error) {
	keys := map[string]string{}
	var order []string

	if strings.TrimSpace(front) == "" {
		return keys, order, nil
	}

	var node yaml.Node
	if err := yaml.Unmarshal([]byte(front), &node); err != nil {
		return nil, nil, err
	}
	if len(node.Content) == 0 {
		return keys, order, nil
	}
	root := node.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, nil, fmt.Errorf("frontmatter must be a YAML mapping")
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		k := root.Content[i].Value
		v := root.Content[i+1].Value
		keys[k] = v
		order = append(order, k)
	}
	return keys, order, nil
}
