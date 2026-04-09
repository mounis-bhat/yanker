package main

import "testing"

func TestMaskPreview(t *testing.T) {
	tests := []struct {
		name  string
		entry Entry
		want  string
	}{
		{
			name:  "plain values are unchanged",
			entry: Entry{Value: "visible", Meta: Meta{Kind: kindPlain}},
			want:  "visible",
		},
		{
			name:  "empty secrets show placeholder",
			entry: Entry{Value: "", Meta: Meta{Kind: kindSecret}},
			want:  "(empty secret)",
		},
		{
			name:  "short secrets are fully masked",
			entry: Entry{Value: "abcd", Meta: Meta{Kind: kindSecret}},
			want:  "****",
		},
		{
			name:  "long secrets keep edges visible",
			entry: Entry{Value: "abcdefghij", Meta: Meta{Kind: kindSecret}},
			want:  "abcd****ghij",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := maskPreview(tt.entry); got != tt.want {
				t.Fatalf("maskPreview() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name  string
		value string
		width int
		want  string
	}{
		{name: "non-positive width returns empty", value: "hello", width: 0, want: ""},
		{name: "short strings are unchanged", value: "hello", width: 5, want: "hello"},
		{name: "width one keeps first rune", value: "hello", width: 1, want: "h"},
		{name: "truncates with ellipsis", value: "hello", width: 4, want: "hel…"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := truncate(tt.value, tt.width); got != tt.want {
				t.Fatalf("truncate() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTruncateLines(t *testing.T) {
	value := "one\ntwo\nthree"
	if got := truncateLines(value, 2); got != "one\ntwo" {
		t.Fatalf("truncateLines() = %q, want %q", got, "one\ntwo")
	}
	if got := truncateLines(value, 5); got != value {
		t.Fatalf("truncateLines() = %q, want %q", got, value)
	}
}
