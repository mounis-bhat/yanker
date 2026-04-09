package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestStoragePath(t *testing.T) {
	t.Run("uses XDG_DATA_HOME when set", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", "/tmp/xdg-home")
		path, err := storagePath()
		if err != nil {
			t.Fatalf("storagePath() error = %v", err)
		}
		want := filepath.Join("/tmp/xdg-home", "yanker", "entries.json")
		if path != want {
			t.Fatalf("storagePath() = %q, want %q", path, want)
		}
	})

	t.Run("falls back to home local share", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("XDG_DATA_HOME", "")
		t.Setenv("HOME", home)
		path, err := storagePath()
		if err != nil {
			t.Fatalf("storagePath() error = %v", err)
		}
		want := filepath.Join(home, ".local", "share", "yanker", "entries.json")
		if path != want {
			t.Fatalf("storagePath() = %q, want %q", path, want)
		}
	})
}

func TestLoadEntries(t *testing.T) {
	t.Run("missing file returns nil entries", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "missing.json")
		entries, err := loadEntries(path)
		if err != nil {
			t.Fatalf("loadEntries() error = %v", err)
		}
		if entries != nil {
			t.Fatalf("loadEntries() = %#v, want nil", entries)
		}
	})

	t.Run("empty file returns nil entries", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "entries.json")
		if err := os.WriteFile(path, []byte("   \n"), 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		entries, err := loadEntries(path)
		if err != nil {
			t.Fatalf("loadEntries() error = %v", err)
		}
		if entries != nil {
			t.Fatalf("loadEntries() = %#v, want nil", entries)
		}
	})

	t.Run("normalizes and sorts loaded entries", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "entries.json")
		data := []byte(`[
			{"key":" zeta ","value":"2","meta":{"kind":" SECRET ","description":" second "}},
			{"key":"alpha","value":"1","meta":{"description":" first "}}
		]`)
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		got, err := loadEntries(path)
		if err != nil {
			t.Fatalf("loadEntries() error = %v", err)
		}

		want := []Entry{
			{Key: "alpha", Value: "1", Meta: Meta{Kind: kindPlain, Description: "first"}},
			{Key: "zeta", Value: "2", Meta: Meta{Kind: kindSecret, Description: "second"}},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("loadEntries() = %#v, want %#v", got, want)
		}
	})

	t.Run("rejects invalid json", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "entries.json")
		if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		if _, err := loadEntries(path); err == nil {
			t.Fatal("loadEntries() error = nil, want invalid JSON error")
		}
	})

	t.Run("rejects duplicate keys", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "entries.json")
		data := []byte(`[
			{"key":"dup","value":"1"},
			{"key":"dup","value":"2"}
		]`)
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		if _, err := loadEntries(path); err == nil {
			t.Fatal("loadEntries() error = nil, want duplicate key error")
		}
	})
}

func TestSaveEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "entries.json")
	entries := []Entry{
		{Key: "beta", Value: "2", Meta: Meta{Kind: kindSecret, Description: "second"}},
		{Key: "alpha", Value: "1", Meta: Meta{Kind: kindPlain, Description: "first"}},
	}

	if err := saveEntries(path, entries); err != nil {
		t.Fatalf("saveEntries() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Fatal("saved file is missing trailing newline")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("saved file perms = %o, want 600", perm)
	}

	loaded, err := loadEntries(path)
	if err != nil {
		t.Fatalf("loadEntries() error = %v", err)
	}
	want := []Entry{
		{Key: "alpha", Value: "1", Meta: Meta{Kind: kindPlain, Description: "first"}},
		{Key: "beta", Value: "2", Meta: Meta{Kind: kindSecret, Description: "second"}},
	}
	if !reflect.DeepEqual(loaded, want) {
		t.Fatalf("round-trip entries = %#v, want %#v", loaded, want)
	}
}
