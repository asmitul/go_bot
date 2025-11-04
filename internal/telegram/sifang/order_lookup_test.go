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
			parts:    []string{"Order ABC1234567 ready", "abc1234567 again", "Ref XY9Z01ABCD"},
			expected: []string{"ABC1234567", "XY9Z01ABCD"},
		},
		{
			name:     "accepts digits only and ignores letters only",
			parts:    []string{"numbers 1234567890 only", "letters ABCDEFGHIJ only"},
			expected: []string{"1234567890"},
		},
		{
			name:     "mix of lower and upper case",
			parts:    []string{"code ab12cd34ef and XY34ZT78AB"},
			expected: []string{"ab12cd34ef", "XY34ZT78AB"},
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
