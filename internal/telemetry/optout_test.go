package telemetry

import (
	"testing"
)

func TestResolveOptOut_FlagBeatsEverything(t *testing.T) {
	yes := true
	sigs := OptOutSignals{
		NoTelemetryFlag:        true,
		SpecScoreTelemetryZero: false,
		DoNotTrack:             false,
		CIDetected:             false,
		PersistentState:        State{Enabled: &yes, UsageStats: &yes, CrashReports: &yes},
	}
	out := ResolveOptOut(sigs, []ChannelName{ChannelUsageStats, ChannelCrashReports})
	for _, name := range []ChannelName{ChannelUsageStats, ChannelCrashReports} {
		d := out[name]
		if d.Enabled {
			t.Errorf("%s should be disabled when --no-telemetry set", name)
		}
		if d.Source != OptOutSourceFlag {
			t.Errorf("%s source: got %q, want %q", name, d.Source, OptOutSourceFlag)
		}
	}
}

func TestResolveOptOut_EnvVarBeatsCIBeatsState(t *testing.T) {
	yes := true
	cases := []struct {
		name   string
		sigs   OptOutSignals
		source OptOutSource
	}{
		{
			name: "SPECSCORE_TELEMETRY=0",
			sigs: OptOutSignals{
				SpecScoreTelemetryZero: true,
				CIDetected:             true,
				PersistentState:        State{Enabled: &yes},
			},
			source: OptOutSourceEnvVar,
		},
		{
			name: "DO_NOT_TRACK=1",
			sigs: OptOutSignals{
				DoNotTrack:      true,
				CIDetected:      true,
				PersistentState: State{Enabled: &yes},
			},
			source: OptOutSourceEnvVar,
		},
		{
			name: "CI=true (no env opt-out)",
			sigs: OptOutSignals{
				CIDetected:      true,
				PersistentState: State{Enabled: &yes},
			},
			source: OptOutSourceCI,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := ResolveOptOut(tc.sigs, []ChannelName{ChannelUsageStats, ChannelCrashReports})
			for _, ch := range []ChannelName{ChannelUsageStats, ChannelCrashReports} {
				d := out[ch]
				if d.Enabled {
					t.Errorf("%s should be disabled", ch)
				}
				if d.Source != tc.source {
					t.Errorf("%s source: got %q, want %q", ch, d.Source, tc.source)
				}
			}
		})
	}
}

func TestResolveOptOut_MalformedStateDisablesAll(t *testing.T) {
	sigs := OptOutSignals{
		PersistentStateInvalid: true,
	}
	out := ResolveOptOut(sigs, []ChannelName{ChannelUsageStats, ChannelCrashReports})
	for _, ch := range []ChannelName{ChannelUsageStats, ChannelCrashReports} {
		d := out[ch]
		if d.Enabled {
			t.Errorf("%s should be disabled when persistent state invalid", ch)
		}
		if d.Source != OptOutSourcePersistentBroken {
			t.Errorf("%s source: got %q, want %q", ch, d.Source, OptOutSourcePersistentBroken)
		}
	}
}

func TestResolveOptOut_PersistentPerChannelOverridesGlobal(t *testing.T) {
	no, yes := false, true
	sigs := OptOutSignals{
		PersistentState: State{Enabled: &no, CrashReports: &yes},
	}
	out := ResolveOptOut(sigs, []ChannelName{ChannelUsageStats, ChannelCrashReports})

	if out[ChannelUsageStats].Enabled {
		t.Errorf("usage-stats should be disabled (global enabled=false)")
	}
	if out[ChannelUsageStats].Source != OptOutSourcePersistentState {
		t.Errorf("usage-stats source: got %q", out[ChannelUsageStats].Source)
	}
	if !out[ChannelCrashReports].Enabled {
		t.Errorf("crash-reports should be enabled (per-channel override)")
	}
	if out[ChannelCrashReports].Source != OptOutSourcePersistentState {
		t.Errorf("crash-reports source: got %q", out[ChannelCrashReports].Source)
	}
}

func TestResolveOptOut_DefaultEnabledWhenNothingSet(t *testing.T) {
	out := ResolveOptOut(OptOutSignals{}, []ChannelName{ChannelUsageStats, ChannelCrashReports})
	for _, ch := range []ChannelName{ChannelUsageStats, ChannelCrashReports} {
		d := out[ch]
		if !d.Enabled {
			t.Errorf("%s should be enabled by default", ch)
		}
		if d.Source != OptOutSourceDefault {
			t.Errorf("%s source: got %q, want %q", ch, d.Source, OptOutSourceDefault)
		}
	}
}

func TestCollectOSEnvSignals_RecognisedMarkers(t *testing.T) {
	cases := []struct {
		name     string
		env      map[string]string
		wantZero bool
		wantDNT  bool
		wantCI   bool
	}{
		{
			name:    "empty",
			env:     map[string]string{},
			wantZero: false, wantDNT: false, wantCI: false,
		},
		{
			name:     "SPECSCORE_TELEMETRY=0",
			env:      map[string]string{"SPECSCORE_TELEMETRY": "0"},
			wantZero: true,
		},
		{
			name:    "DO_NOT_TRACK=1",
			env:     map[string]string{"DO_NOT_TRACK": "1"},
			wantDNT: true,
		},
		{
			name:   "CI=true",
			env:    map[string]string{"CI": "true"},
			wantCI: true,
		},
		{
			name:   "GITHUB_ACTIONS=true",
			env:    map[string]string{"GITHUB_ACTIONS": "true"},
			wantCI: true,
		},
		{
			name:   "CI=1 should NOT trigger (case-sensitive value)",
			env:    map[string]string{"CI": "1"},
			wantCI: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			getEnv := func(k string) string { return tc.env[k] }
			zero, dnt, ci := CollectOSEnvSignals(getEnv)
			if zero != tc.wantZero {
				t.Errorf("zero: got %v, want %v", zero, tc.wantZero)
			}
			if dnt != tc.wantDNT {
				t.Errorf("dnt: got %v, want %v", dnt, tc.wantDNT)
			}
			if ci != tc.wantCI {
				t.Errorf("ci: got %v, want %v", ci, tc.wantCI)
			}
		})
	}
}
