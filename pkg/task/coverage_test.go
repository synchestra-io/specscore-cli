package task

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// ParseBoard — no separator line
// ---------------------------------------------------------------------------

func TestParseBoard_NoSeparator(t *testing.T) {
	data := []byte("# Tasks\n\nNo table here.\n")
	_, err := ParseBoard(data)
	if err == nil {
		t.Fatal("expected error for missing table separator")
	}
	if !strings.Contains(err.Error(), "no table separator") {
		t.Errorf("error = %v", err)
	}
}

// ---------------------------------------------------------------------------
// ParseBoard — row with wrong column count
// ---------------------------------------------------------------------------

func TestParseBoard_WrongColumnCount(t *testing.T) {
	data := []byte("| A | B | C |\n|---|---|---|\n| one | two | three |\n")
	_, err := ParseBoard(data)
	if err == nil {
		t.Fatal("expected error for wrong column count")
	}
	if !strings.Contains(err.Error(), "expected 7 columns") {
		t.Errorf("error = %v", err)
	}
}

// ---------------------------------------------------------------------------
// ParseBoard — invalid status cell (no backticks)
// ---------------------------------------------------------------------------

func TestParseBoard_InvalidStatusCell(t *testing.T) {
	// 7 columns but invalid status (no backticks)
	data := []byte("| A | B | C | D | E | F | G |\n|---|---|---|---|---|---|---|\n| [task](task/) | badstatus | — | — | — | — | — |\n")
	_, err := ParseBoard(data)
	if err == nil {
		t.Fatal("expected error for invalid status cell")
	}
	if !strings.Contains(err.Error(), "invalid status cell") {
		t.Errorf("error = %v", err)
	}
}

// ---------------------------------------------------------------------------
// ParseBoard — unknown status name in backticks
// ---------------------------------------------------------------------------

func TestParseBoard_UnknownStatus(t *testing.T) {
	data := []byte("| A | B | C | D | E | F | G |\n|---|---|---|---|---|---|---|\n| [task](task/) | ❓ `unknown-status` | — | — | — | — | — |\n")
	_, err := ParseBoard(data)
	if err == nil {
		t.Fatal("expected error for unknown status")
	}
	if !strings.Contains(err.Error(), "unknown status") {
		t.Errorf("error = %v", err)
	}
}

// ---------------------------------------------------------------------------
// ParseBoard — empty lines and non-table lines after separator are skipped
// ---------------------------------------------------------------------------

func TestParseBoard_SkipsEmptyAndNonTableLines(t *testing.T) {
	data := []byte("| A | B | C | D | E | F | G |\n|---|---|---|---|---|---|---|\n\nNot a table line\n| [task](task/) | ✅ `completed` | — | — | — | — | — |\n")
	bv, err := ParseBoard(data)
	if err != nil {
		t.Fatalf("ParseBoard: %v", err)
	}
	if len(bv.Rows) != 1 {
		t.Errorf("expected 1 row, got %d", len(bv.Rows))
	}
}

// ---------------------------------------------------------------------------
// ParseStatusCell — missing closing backtick
// ---------------------------------------------------------------------------

func TestParseStatusCell_MissingClosingBacktick(t *testing.T) {
	_, err := ParseStatusCell("✅ `completed")
	if err == nil {
		t.Fatal("expected error for missing closing backtick")
	}
}

// ---------------------------------------------------------------------------
// ParseTaskFile — missing title
// ---------------------------------------------------------------------------

func TestParseTaskFile_MissingTitle(t *testing.T) {
	_, err := ParseTaskFile([]byte("No title line\n"))
	if err == nil {
		t.Fatal("expected error for missing title")
	}
	if !strings.Contains(err.Error(), "missing task title") {
		t.Errorf("error = %v", err)
	}
}

// ---------------------------------------------------------------------------
// ParseTaskFile — title only, no newline (missing Dependencies)
// ---------------------------------------------------------------------------

func TestParseTaskFile_TitleOnlyNoNewline(t *testing.T) {
	_, err := ParseTaskFile([]byte("# Title only"))
	if err == nil {
		t.Fatal("expected error for missing Dependencies")
	}
	if !strings.Contains(err.Error(), "missing Dependencies") {
		t.Errorf("error = %v", err)
	}
}

// ---------------------------------------------------------------------------
// ParseTaskFile — missing Summary section
// ---------------------------------------------------------------------------

func TestParseTaskFile_MissingSummary(t *testing.T) {
	data := []byte("# Title\n\nDescription.\n\n## Dependencies\n\nNone\n")
	_, err := ParseTaskFile(data)
	if err == nil {
		t.Fatal("expected error for missing Summary section")
	}
	if !strings.Contains(err.Error(), "missing Summary") {
		t.Errorf("error = %v", err)
	}
}

// ---------------------------------------------------------------------------
// ParseTaskFile — section heading with no body
// ---------------------------------------------------------------------------

func TestParseTaskFile_SectionNoBody(t *testing.T) {
	data := []byte("# Title\n\nDesc.\n\n## Dependencies\n\nNone\n\n## Summary")
	result, err := ParseTaskFile(data)
	if err != nil {
		t.Fatalf("ParseTaskFile: %v", err)
	}
	if result.Summary != "" {
		t.Errorf("expected empty summary for section with no body, got %q", result.Summary)
	}
}

// ---------------------------------------------------------------------------
// ParseTaskFile — dependencies as "None"
// ---------------------------------------------------------------------------

func TestParseTaskFile_DepsNone(t *testing.T) {
	data := []byte("# My Task\n\nSome desc.\n\n## Dependencies\n\nNone\n\n## Summary\n\nDone.\n")
	result, err := ParseTaskFile(data)
	if err != nil {
		t.Fatalf("ParseTaskFile: %v", err)
	}
	if len(result.DependsOn) != 0 {
		t.Errorf("expected no deps for 'None', got %v", result.DependsOn)
	}
}

// ---------------------------------------------------------------------------
// ParseTaskFile — summary as "None"
// ---------------------------------------------------------------------------

func TestParseTaskFile_SummaryNone(t *testing.T) {
	data := []byte("# Task\n\n## Dependencies\n\nNone\n\n## Summary\n\nNone\n")
	result, err := ParseTaskFile(data)
	if err != nil {
		t.Fatalf("ParseTaskFile: %v", err)
	}
	if result.Summary != "" {
		t.Errorf("expected empty summary for 'None', got %q", result.Summary)
	}
}

// ---------------------------------------------------------------------------
// parseBoardRow — leading/trailing empty cells handling
// ---------------------------------------------------------------------------

func TestParseBoardRow_LeadingTrailingPipes(t *testing.T) {
	// Normal row with leading/trailing pipes
	row, err := parseBoardRow("| [task](task/) | ✅ `completed` | dep1, dep2 | `branch` | agent | @req | 2026-01-01 |")
	if err != nil {
		t.Fatalf("parseBoardRow: %v", err)
	}
	if row.Task != "task" {
		t.Errorf("Task = %q", row.Task)
	}
	if row.Status != StatusCompleted {
		t.Errorf("Status = %q", row.Status)
	}
	if len(row.DependsOn) != 2 {
		t.Errorf("DependsOn = %v", row.DependsOn)
	}
	if row.Branch != "branch" {
		t.Errorf("Branch = %q", row.Branch)
	}
}
