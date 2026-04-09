package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestNormalizeAddArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "flags before positional args remain first",
			args: []string{"-kind", "secret", "-desc", "GitHub token", "gh-token", "value"},
			want: []string{"-kind", "secret", "-desc", "GitHub token", "gh-token", "value"},
		},
		{
			name: "flags after positional args are moved first",
			args: []string{"gh-token", "value", "-kind", "secret", "-desc", "GitHub token"},
			want: []string{"-kind", "secret", "-desc", "GitHub token", "gh-token", "value"},
		},
		{
			name: "unknown flag-like args preserve current parsing behavior",
			args: []string{"key", "value", "-extra", "flag"},
			want: []string{"-extra", "key", "value", "flag"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeAddArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("normalizeAddArgs() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestRunGet(t *testing.T) {
	entries := []Entry{{Key: "gh-token", Value: "secret-value"}}

	t.Run("requires exactly one arg", func(t *testing.T) {
		err := runGet(entries, nil)
		if err == nil || err.Error() != "usage: yanker get <key>" {
			t.Fatalf("runGet() error = %v, want usage error", err)
		}
	})

	t.Run("returns missing key error", func(t *testing.T) {
		err := runGet(entries, []string{"missing"})
		if err == nil || err.Error() != `no entry found for "missing"` {
			t.Fatalf("runGet() error = %v, want missing key error", err)
		}
	})

	t.Run("prints raw value", func(t *testing.T) {
		stdout := captureStdout(t, func() {
			if err := runGet(entries, []string{"gh-token"}); err != nil {
				t.Fatalf("runGet() error = %v", err)
			}
		})
		if stdout != "secret-value" {
			t.Fatalf("stdout = %q, want %q", stdout, "secret-value")
		}
	})
}

func TestRunAdd(t *testing.T) {
	path := filepath.Join(t.TempDir(), "entries.json")
	entries := []Entry{{Key: "existing", Value: "old", Meta: Meta{Kind: kindPlain}}}

	stdout := captureStdout(t, func() {
		if err := runAdd(path, entries, []string{"new-key", "new-value", "-kind", "secret", "-desc", " new desc "}); err != nil {
			t.Fatalf("runAdd() error = %v", err)
		}
	})
	if stdout != "added new-key\n" {
		t.Fatalf("stdout = %q, want %q", stdout, "added new-key\n")
	}

	loaded, err := loadEntries(path)
	if err != nil {
		t.Fatalf("loadEntries() error = %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("loaded entries len = %d, want 2", len(loaded))
	}
	if got := loaded[1]; got.Key != "new-key" || got.Value != "new-value" || got.Meta.Kind != kindSecret || got.Meta.Description != "new desc" {
		t.Fatalf("loaded entry = %#v, want normalized added entry", got)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe() error = %v", err)
	}
	os.Stdout = w
	defer func() {
		os.Stdout = original
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("Copy() error = %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return buf.String()
}
