package telemetry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadState_AbsentFile_NoError_NoFile(t *testing.T) {
	withTempHome(t)
	r, err := ReadState()
	if err != nil {
		t.Fatalf("ReadState on absent file: %v", err)
	}
	if r.FileExisted {
		t.Errorf("FileExisted=true on absent file")
	}
	if r.InvalidReason != "" {
		t.Errorf("InvalidReason should be empty on absent file, got %q", r.InvalidReason)
	}
}

func TestReadState_ValidFile_Parsed(t *testing.T) {
	home := withTempHome(t)
	dir := filepath.Join(home, ".specscore")
	if err := os.MkdirAll(dir, installIDDirMode); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := stateSchemaPointer + "\nenabled: false\ncrash-reports: true\n"
	if err := os.WriteFile(filepath.Join(dir, stateFilename), []byte(body), stateFileMode); err != nil {
		t.Fatalf("write: %v", err)
	}

	r, err := ReadState()
	if err != nil {
		t.Fatalf("ReadState: %v", err)
	}
	if !r.FileExisted {
		t.Fatalf("FileExisted=false on present file")
	}
	if r.InvalidReason != "" {
		t.Fatalf("InvalidReason should be empty, got %q", r.InvalidReason)
	}
	if r.State.Enabled == nil || *r.State.Enabled != false {
		t.Errorf("Enabled should be *false, got %v", r.State.Enabled)
	}
	if r.State.CrashReports == nil || *r.State.CrashReports != true {
		t.Errorf("CrashReports should be *true, got %v", r.State.CrashReports)
	}
	if r.State.UsageStats != nil {
		t.Errorf("UsageStats should be nil (absent key), got %v", *r.State.UsageStats)
	}
}

func TestReadState_UnknownKey_InvalidReason(t *testing.T) {
	home := withTempHome(t)
	dir := filepath.Join(home, ".specscore")
	if err := os.MkdirAll(dir, installIDDirMode); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := stateSchemaPointer + "\nanalytics_provider: posthog\n"
	if err := os.WriteFile(filepath.Join(dir, stateFilename), []byte(body), stateFileMode); err != nil {
		t.Fatalf("write: %v", err)
	}

	r, err := ReadState()
	if err != nil {
		t.Fatalf("ReadState should not error on malformed content: %v", err)
	}
	if !r.FileExisted {
		t.Fatalf("FileExisted=false on present file")
	}
	if r.InvalidReason == "" {
		t.Fatalf("InvalidReason should be set for unknown key")
	}
	if !strings.Contains(r.InvalidReason, "analytics_provider") {
		t.Errorf("InvalidReason should name the offending key, got %q", r.InvalidReason)
	}
}

func TestReadState_YamlSyntaxError_InvalidReason(t *testing.T) {
	home := withTempHome(t)
	dir := filepath.Join(home, ".specscore")
	if err := os.MkdirAll(dir, installIDDirMode); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := stateSchemaPointer + "\nenabled: not\n  : yaml: at all\n"
	if err := os.WriteFile(filepath.Join(dir, stateFilename), []byte(body), stateFileMode); err != nil {
		t.Fatalf("write: %v", err)
	}

	r, err := ReadState()
	if err != nil {
		t.Fatalf("ReadState should not error: %v", err)
	}
	if r.InvalidReason == "" {
		t.Errorf("InvalidReason should be set for syntax error")
	}
}

func TestWriteState_RoundTripsAndIncludesSchemaPointer(t *testing.T) {
	home := withTempHome(t)
	yes := true
	no := false
	in := State{
		Enabled:      &yes,
		CrashReports: &no,
	}
	if err := WriteState(in); err != nil {
		t.Fatalf("WriteState: %v", err)
	}

	// File contents start with the schema pointer.
	path := filepath.Join(home, ".specscore", stateFilename)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.HasPrefix(string(raw), stateSchemaPointer+"\n") {
		t.Errorf("file does not start with schema pointer; first line: %q",
			strings.SplitN(string(raw), "\n", 2)[0])
	}

	// Round-trip back to State.
	r, err := ReadState()
	if err != nil {
		t.Fatalf("ReadState: %v", err)
	}
	if r.InvalidReason != "" {
		t.Errorf("round-trip InvalidReason: %q", r.InvalidReason)
	}
	if r.State.Enabled == nil || *r.State.Enabled != true {
		t.Errorf("Enabled lost in round-trip")
	}
	if r.State.CrashReports == nil || *r.State.CrashReports != false {
		t.Errorf("CrashReports lost in round-trip")
	}
}

func TestChannelEnabled_PrecedenceAndSource(t *testing.T) {
	yes, no := true, false

	cases := []struct {
		name         string
		state        State
		channel      ChannelName
		wantEnabled  bool
		wantSourceIs string // substring match
	}{
		{
			name:         "default-no-preference",
			state:        State{},
			channel:      ChannelUsageStats,
			wantEnabled:  true,
			wantSourceIs: "default",
		},
		{
			name:         "global-only-disabled",
			state:        State{Enabled: &no},
			channel:      ChannelUsageStats,
			wantEnabled:  false,
			wantSourceIs: "global",
		},
		{
			name:         "per-channel-overrides-global",
			state:        State{Enabled: &no, CrashReports: &yes},
			channel:      ChannelCrashReports,
			wantEnabled:  true,
			wantSourceIs: "per-channel",
		},
		{
			name:         "per-channel-disabled-overrides-global-enabled",
			state:        State{Enabled: &yes, UsageStats: &no},
			channel:      ChannelUsageStats,
			wantEnabled:  false,
			wantSourceIs: "per-channel",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, src := tc.state.ChannelEnabled(tc.channel)
			if got != tc.wantEnabled {
				t.Errorf("enabled: got %v, want %v", got, tc.wantEnabled)
			}
			if !strings.Contains(src, tc.wantSourceIs) {
				t.Errorf("source: got %q, want substring %q", src, tc.wantSourceIs)
			}
		})
	}
}
