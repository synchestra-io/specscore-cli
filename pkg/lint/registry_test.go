package lint

import (
	"testing"
)

// TestParseConsumerPath exercises every input row from
// [cli/spec/lint#ac:consumer-path-multi-glob-parsed] and
// [cli/spec/lint#req:consumer-path-multi-glob].
func TestParseConsumerPath(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{
			name: "single glob",
			in:   "spec/features/**/*.entity.md",
			want: []string{"spec/features/**/*.entity.md"},
		},
		{
			name: "two globs no whitespace",
			in:   "a,b",
			want: []string{"a", "b"},
		},
		{
			name: "two globs whitespace after comma",
			in:   "spec/features/**/*.entity.md, spec/features/**/*.property.md",
			want: []string{"spec/features/**/*.entity.md", "spec/features/**/*.property.md"},
		},
		{
			name: "two globs whitespace before comma",
			in:   "spec/features/**/*.entity.md ,spec/features/**/*.property.md",
			want: []string{"spec/features/**/*.entity.md", "spec/features/**/*.property.md"},
		},
		{
			name: "two globs whitespace both sides of comma",
			in:   "a , b",
			want: []string{"a", "b"},
		},
		{
			name: "empty cell",
			in:   "",
			want: nil,
		},
		{
			name: "em dash placeholder",
			in:   "—",
			want: nil,
		},
		{
			name: "ascii hyphen placeholder",
			in:   "-",
			want: nil,
		},
		{
			name: "leading comma discarded",
			in:   ",a,b",
			want: []string{"a", "b"},
		},
		{
			name: "trailing comma discarded",
			in:   "a,b,",
			want: []string{"a", "b"},
		},
		{
			name: "doubled internal comma discarded",
			in:   "a,,b",
			want: []string{"a", "b"},
		},
		{
			name: "all-empty commas and whitespace",
			in:   ",, ,",
			want: nil,
		},
		{
			name: "whitespace-only cell",
			in:   "   ",
			want: nil,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := parseConsumerPath(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("parseConsumerPath(%q) = %v (len %d), want %v (len %d)",
					tc.in, got, len(got), tc.want, len(tc.want))
			}
			if tc.want == nil && got != nil {
				t.Fatalf("parseConsumerPath(%q) = %v (non-nil), want nil", tc.in, got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("parseConsumerPath(%q)[%d] = %q, want %q",
						tc.in, i, got[i], tc.want[i])
				}
			}
		})
	}
}
