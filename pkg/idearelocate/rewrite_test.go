package idearelocate

import (
	"strings"
	"testing"
)

// Unit tests for RewriteBody, the in-file substitution function from
// Task 3 of the relocate Plan. Two rules to verify:
//
//  1. synchestra-io/ → specscore/ (org rename, applied globally).
//  2. Word-bounded "this repo" (case-insensitive) → target slug,
//     EXCEPT inside fenced code blocks, inline code spans, or table
//     cells.

func TestRewriteBody_OrgRenameSingle(t *testing.T) {
	in := "Link to synchestra-io/specscore for the worked example.\n"
	out := RewriteBody(in, "specscore")
	if !strings.Contains(out, "specscore/specscore") {
		t.Errorf("expected synchestra-io/specscore → specscore/specscore; got: %q", out)
	}
	if strings.Contains(out, "synchestra-io/") {
		t.Errorf("expected synchestra-io/ to be fully replaced; got: %q", out)
	}
}

func TestRewriteBody_OrgRenameMultipleRepos(t *testing.T) {
	in := "See synchestra-io/specscore-cli and synchestra-io/specstudio-skills.\n"
	out := RewriteBody(in, "specscore")
	if strings.Contains(out, "synchestra-io/") {
		t.Errorf("expected all synchestra-io/ rewritten; got: %q", out)
	}
	if !strings.Contains(out, "specscore/specscore-cli") || !strings.Contains(out, "specscore/specstudio-skills") {
		t.Errorf("expected both repos rewritten; got: %q", out)
	}
}

func TestRewriteBody_ThisRepoInProse(t *testing.T) {
	in := "Existing artifacts in this repo are migrated.\n"
	out := RewriteBody(in, "specscore")
	want := "Existing artifacts in specscore are migrated."
	if !strings.Contains(out, want) {
		t.Errorf("expected %q in output; got: %q", want, out)
	}
}

func TestRewriteBody_ThisRepoCaseInsensitive(t *testing.T) {
	cases := []string{"This Repo", "THIS REPO", "this REPO"}
	for _, c := range cases {
		in := c + " is the source.\n"
		out := RewriteBody(in, "specscore")
		if !strings.Contains(out, "specscore is the source") {
			t.Errorf("case %q: expected rewrite; got: %q", c, out)
		}
	}
}

func TestRewriteBody_ThisRepoWordBoundary(t *testing.T) {
	// "this repository" must NOT match "this repo".
	in := "This repository should not match. This repos should not match.\n"
	out := RewriteBody(in, "specscore")
	if !strings.Contains(out, "This repository should not match") {
		t.Errorf("expected 'this repository' preserved; got: %q", out)
	}
	if !strings.Contains(out, "This repos should not match") {
		t.Errorf("expected 'this repos' preserved; got: %q", out)
	}
}

func TestRewriteBody_ThisRepoInFencedCodeBlockPreserved(t *testing.T) {
	in := "Prose says this repo.\n\n" +
		"```\n" +
		"git -C this-repo status\n" +
		"this repo here\n" +
		"```\n\n" +
		"More this repo prose.\n"
	out := RewriteBody(in, "specscore")

	// Prose outside the fence → rewritten.
	if !strings.Contains(out, "Prose says specscore.") {
		t.Errorf("expected pre-fence prose rewritten; got:\n%s", out)
	}
	if !strings.Contains(out, "More specscore prose.") {
		t.Errorf("expected post-fence prose rewritten; got:\n%s", out)
	}
	// Inside the fence → untouched.
	if !strings.Contains(out, "this repo here") {
		t.Errorf("expected fenced 'this repo' preserved; got:\n%s", out)
	}
	if !strings.Contains(out, "git -C this-repo status") {
		t.Errorf("expected fenced 'this-repo' preserved; got:\n%s", out)
	}
}

func TestRewriteBody_FencedCodeBlockWithLanguageTag(t *testing.T) {
	in := "Outside this repo here.\n" +
		"```bash\nthis repo inside\n```\n" +
		"After this repo here.\n"
	out := RewriteBody(in, "specscore")
	if !strings.Contains(out, "Outside specscore here.") {
		t.Errorf("expected pre-fence rewritten; got:\n%s", out)
	}
	if !strings.Contains(out, "this repo inside") {
		t.Errorf("expected fenced (language-tagged) content preserved; got:\n%s", out)
	}
	if !strings.Contains(out, "After specscore here.") {
		t.Errorf("expected post-fence rewritten; got:\n%s", out)
	}
}

func TestRewriteBody_ThisRepoInInlineCodePreserved(t *testing.T) {
	in := "Use `this repo` in commands; otherwise this repo is wrong.\n"
	out := RewriteBody(in, "specscore")
	if !strings.Contains(out, "`this repo`") {
		t.Errorf("expected inline-code 'this repo' preserved; got: %q", out)
	}
	if !strings.Contains(out, "otherwise specscore is wrong") {
		t.Errorf("expected prose 'this repo' rewritten; got: %q", out)
	}
}

func TestRewriteBody_ThisRepoInTableCellPreserved(t *testing.T) {
	in := "| Term | Meaning |\n" +
		"|---|---|\n" +
		"| this repo | the local repo |\n\n" +
		"This repo is referenced elsewhere.\n"
	out := RewriteBody(in, "specscore")
	if !strings.Contains(out, "| this repo |") {
		t.Errorf("expected table cell preserved; got:\n%s", out)
	}
	if !strings.Contains(out, "specscore is referenced elsewhere") {
		t.Errorf("expected non-table prose rewritten; got:\n%s", out)
	}
}

func TestRewriteBody_OrgRenameAppliesInsideCodeBlocks(t *testing.T) {
	// The org-rename rule has no carve-outs — it applies globally, including
	// inside fenced code blocks. Only the "this repo" rule respects
	// code-context carve-outs.
	in := "```\ngit clone synchestra-io/specscore\n```\n"
	out := RewriteBody(in, "specscore")
	if !strings.Contains(out, "git clone specscore/specscore") {
		t.Errorf("expected org-rename to apply inside fence; got:\n%s", out)
	}
}
