package entity

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestWalk_VisitsEveryDiscoveredEntity(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		filepath.Join("features", "user", "user.entity.md"): `---
kind: entity
id: user
singular: User
plural: Users
properties: []
---

# Entity: User

## Description

Stub.

## Properties

<!-- managed-by: specscore lint --fix -->
<!-- end-managed -->

## Referenced by

<!-- managed-by: specscore lint --fix -->
- _No references yet._
<!-- end-managed -->
`,
		filepath.Join("features", "order", "order.entity.md"): `---
kind: entity
id: order
singular: Order
plural: Orders
properties: []
---

# Entity: Order

## Description

Stub.

## Properties

<!-- managed-by: specscore lint --fix -->
<!-- end-managed -->

## Referenced by

<!-- managed-by: specscore lint --fix -->
- _No references yet._
<!-- end-managed -->
`,
	}
	for rel, content := range files {
		abs := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var seen []string
	err := Walk(root, func(d *Doc) error {
		if d == nil {
			return errors.New("nil doc passed to Walk callback")
		}
		seen = append(seen, d.Slug)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(seen)
	if len(seen) != 2 || seen[0] != "order" || seen[1] != "user" {
		t.Errorf("walked slugs = %v, want [order user]", seen)
	}
}

func TestWalk_PropagatesCallbackError(t *testing.T) {
	root := t.TempDir()
	rel := filepath.Join("features", "user", "user.entity.md")
	abs := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte("# stub\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	sentinel := fmt.Errorf("callback failure")
	err := Walk(root, func(d *Doc) error { return sentinel })
	if !errors.Is(err, sentinel) {
		t.Errorf("Walk err = %v, want %v", err, sentinel)
	}
}

func TestWalk_EmptyTreeIsNotAnError(t *testing.T) {
	root := t.TempDir()
	called := false
	err := Walk(root, func(d *Doc) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("Walk on empty tree returned err: %v", err)
	}
	if called {
		t.Error("callback should not fire on an empty tree")
	}
}
