package sifang

import "testing"

func TestExtractOrderNumbers(t *testing.T) {
	tests := []struct {
		name     string
		parts    []string
		expected []string
	}{
		{
			name:     "deduplicates and preserves order",
			parts:    []string{"Order ABC12345 ready", "abc12345 again", "Ref XY9Z01"},
			expected: []string{"ABC12345", "XY9Z01"},
		},
		{
			name:     "ignores single charset matches",
			parts:    []string{"numbers 123456 only", "letters ABCDEF only"},
			expected: nil,
		},
		{
			name:     "mix of lower and upper case",
			parts:    []string{"code ab12cd and XY34ZT"},
			expected: []string{"ab12cd", "XY34ZT"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractOrderNumbers(tt.parts...)
			if len(got) != len(tt.expected) {
				t.Fatalf("expected %d matches, got %d (%v)", len(tt.expected), len(got), got)
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Fatalf("expected %v, got %v", tt.expected, got)
				}
			}
		})
	}
}

func TestNormalizeFileName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"  IMG-AB12CD.jpg  ", "IMG AB12CD"},
		{"report_XY99Z0.pdf", "report XY99Z0"},
		{"", ""},
	}

	for _, tt := range tests {
		if got := NormalizeFileName(tt.input); got != tt.expected {
			t.Fatalf("NormalizeFileName(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
