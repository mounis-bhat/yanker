package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func storagePath() (string, error) {
	if xdg := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); xdg != "" {
		return filepath.Join(xdg, "yanker", "entries.json"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", errors.New("unable to resolve storage path")
	}
	if home != "" {
		return filepath.Join(home, ".local", "share", "yanker", "entries.json"), nil
	}
	return filepath.Join(home, ".yanker.json"), nil
}

func loadEntries(path string) ([]Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, nil
	}

	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	for i := range entries {
		entries[i].Key = strings.TrimSpace(entries[i].Key)
		entries[i].Meta.Kind = normalizeKind(entries[i].Meta.Kind)
		entries[i].Meta.Description = strings.TrimSpace(entries[i].Meta.Description)
	}
	if err := ensureUniqueKeys(entries); err != nil {
		return nil, err
	}
	sortEntries(entries)
	return entries, nil
}

func saveEntries(path string, entries []Entry) error {
	if err := ensureUniqueKeys(entries); err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, ".entries-*.json")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	shouldRemove := true
	defer func() {
		if !shouldRemove {
			return
		}
		if err := os.Remove(tmpPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "warning: failed to remove temp file %q: %v\n", tmpPath, err)
		}
	}()

	closeTemp := func() error {
		if err := tmp.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			return err
		}
		return nil
	}

	if err := tmp.Chmod(0o600); err != nil {
		if closeErr := closeTemp(); closeErr != nil {
			return fmt.Errorf("chmod temp file: %w (close temp file: %v)", err, closeErr)
		}
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		if closeErr := closeTemp(); closeErr != nil {
			return fmt.Errorf("write temp file: %w (close temp file: %v)", err, closeErr)
		}
		return err
	}
	if err := closeTemp(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	shouldRemove = false
	return nil
}
