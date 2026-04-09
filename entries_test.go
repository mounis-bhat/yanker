package main

import (
	"reflect"
	"testing"
)

func TestEnsureUniqueKeys(t *testing.T) {
	tests := []struct {
		name    string
		entries []Entry
		wantErr string
	}{
		{
			name: "accepts unique trimmed keys",
			entries: []Entry{
				{Key: " alpha "},
				{Key: "beta"},
			},
		},
		{
			name: "rejects empty keys",
			entries: []Entry{
				{Key: "   "},
			},
			wantErr: "entries must have non-empty keys",
		},
		{
			name: "rejects duplicate keys",
			entries: []Entry{
				{Key: "alpha"},
				{Key: "alpha"},
			},
			wantErr: `duplicate key "alpha"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ensureUniqueKeys(tt.entries)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ensureUniqueKeys() error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("ensureUniqueKeys() error = nil, want %q", tt.wantErr)
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("ensureUniqueKeys() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestValidateEntryField(t *testing.T) {
	existing := []Entry{{Key: "existing"}}

	tests := []struct {
		name        string
		entry       Entry
		entries     []Entry
		originalKey string
		wantField   int
		wantErr     string
	}{
		{
			name:      "requires key",
			entry:     Entry{Value: "value", Meta: Meta{Kind: kindPlain}},
			wantField: fieldKey,
			wantErr:   "key is required",
		},
		{
			name:      "requires value",
			entry:     Entry{Key: "alpha", Meta: Meta{Kind: kindPlain}},
			wantField: fieldValue,
			wantErr:   "value is required",
		},
		{
			name:      "rejects invalid kind",
			entry:     Entry{Key: "alpha", Value: "value", Meta: Meta{Kind: "invalid"}},
			wantField: fieldKind,
			wantErr:   `invalid kind "invalid"`,
		},
		{
			name:      "rejects duplicate key",
			entry:     Entry{Key: "existing", Value: "value", Meta: Meta{Kind: kindPlain}},
			entries:   existing,
			wantField: fieldKey,
			wantErr:   `key "existing" already exists`,
		},
		{
			name:        "allows original key while editing",
			entry:       Entry{Key: "existing", Value: "value", Meta: Meta{Kind: kindSecret}},
			entries:     existing,
			originalKey: "existing",
			wantField:   -1,
			wantErr:     "",
		},
		{
			name:      "normalizes plain kind and trims fields",
			entry:     Entry{Key: " alpha ", Value: "value", Meta: Meta{Kind: " PLAIN ", Description: " desc "}},
			wantField: -1,
			wantErr:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, err := validateEntryField(tt.entry, tt.entries, tt.originalKey)
			if field != tt.wantField {
				t.Fatalf("validateEntryField() field = %d, want %d", field, tt.wantField)
			}
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateEntryField() error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validateEntryField() error = nil, want %q", tt.wantErr)
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("validateEntryField() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestNormalizeKind(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "", want: kindPlain},
		{input: " plain ", want: kindPlain},
		{input: " SECRET ", want: kindSecret},
		{input: "CuStOm", want: "custom"},
	}

	for _, tt := range tests {
		if got := normalizeKind(tt.input); got != tt.want {
			t.Fatalf("normalizeKind(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFilterEntriesRanking(t *testing.T) {
	entries := []Entry{
		{Key: "github", Meta: Meta{Description: "code hosting"}},
		{Key: "gitlab", Meta: Meta{Description: "team code"}},
		{Key: "db-prod", Meta: Meta{Description: "database production"}},
		{Key: "alpha", Meta: Meta{Description: "query"}},
		{Key: "beta", Meta: Meta{Description: "query"}},
	}

	t.Run("empty query sorts alphabetically", func(t *testing.T) {
		got := filterEntries(entries, "")
		want := []string{"alpha", "beta", "db-prod", "github", "gitlab"}
		assertKeys(t, got, want)
	})

	t.Run("exact key beats prefix and fuzzy", func(t *testing.T) {
		got := filterEntries(entries, "git")
		want := []string{"github", "gitlab"}
		assertKeys(t, got[:2], want)
	})

	t.Run("description matches are included", func(t *testing.T) {
		got := filterEntries(entries, "prod")
		want := []string{"db-prod"}
		assertKeys(t, got, want)
	})

	t.Run("alphabetical fallback breaks equal scores", func(t *testing.T) {
		got := filterEntries(entries, "query")
		want := []string{"alpha", "beta"}
		assertKeys(t, got[:2], want)
	})

	t.Run("returns no matches when query misses", func(t *testing.T) {
		got := filterEntries(entries, "zzz")
		if len(got) != 0 {
			t.Fatalf("filterEntries() len = %d, want 0", len(got))
		}
	})
}

func TestSubsequenceScore(t *testing.T) {
	if _, ok := subsequenceScore("alphabet", "az"); ok {
		t.Fatal("subsequenceScore() matched impossible query")
	}

	contiguous, ok := subsequenceScore("alphabet", "alp")
	if !ok {
		t.Fatal("subsequenceScore() did not match contiguous query")
	}

	gapped, ok := subsequenceScore("alphabet", "abt")
	if !ok {
		t.Fatal("subsequenceScore() did not match gapped query")
	}

	if contiguous <= gapped {
		t.Fatalf("subsequenceScore() contiguous = %d, gapped = %d, want contiguous > gapped", contiguous, gapped)
	}
}

func assertKeys(t *testing.T, entries []Entry, want []string) {
	t.Helper()
	got := make([]string, 0, len(entries))
	for _, entry := range entries {
		got = append(got, entry.Key)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("keys = %#v, want %#v", got, want)
	}
}
