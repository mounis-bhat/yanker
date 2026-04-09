package main

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

func ensureUniqueKeys(entries []Entry) error {
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		key := strings.TrimSpace(entry.Key)
		if key == "" {
			return errors.New("entries must have non-empty keys")
		}
		if _, ok := seen[key]; ok {
			return fmt.Errorf("duplicate key %q", key)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func validateEntry(entry Entry, entries []Entry, originalKey string) error {
	_, err := validateEntryField(entry, entries, originalKey)
	return err
}

func validateEntryField(entry Entry, entries []Entry, originalKey string) (int, error) {
	entry.Key = strings.TrimSpace(entry.Key)
	entry.Meta.Description = strings.TrimSpace(entry.Meta.Description)
	entry.Meta.Kind = normalizeKind(entry.Meta.Kind)

	if entry.Key == "" {
		return fieldKey, errors.New("key is required")
	}
	if entry.Value == "" {
		return fieldValue, errors.New("value is required")
	}
	if entry.Meta.Kind != kindPlain && entry.Meta.Kind != kindSecret {
		return fieldKind, fmt.Errorf("invalid kind %q", entry.Meta.Kind)
	}
	for _, existing := range entries {
		if existing.Key == entry.Key && existing.Key != originalKey {
			return fieldKey, fmt.Errorf("key %q already exists", entry.Key)
		}
	}
	return -1, nil
}

func normalizeKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "", kindPlain:
		return kindPlain
	case kindSecret:
		return kindSecret
	default:
		return strings.ToLower(strings.TrimSpace(kind))
	}
}

func sortEntries(entries []Entry) {
	sort.Slice(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].Key) < strings.ToLower(entries[j].Key)
	})
}

func findEntry(entries []Entry, key string) (Entry, bool) {
	for _, entry := range entries {
		if entry.Key == key {
			return entry, true
		}
	}
	return Entry{}, false
}

func filterEntries(entries []Entry, query string) []Entry {
	query = strings.TrimSpace(strings.ToLower(query))
	if query == "" {
		result := append([]Entry(nil), entries...)
		sortEntries(result)
		return result
	}

	scored := make([]scoredEntry, 0, len(entries))
	for idx, entry := range entries {
		if score, ok := matchScore(entry, query); ok {
			scored = append(scored, scoredEntry{entry: entry, score: score, idx: idx})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		left := strings.ToLower(scored[i].entry.Key)
		right := strings.ToLower(scored[j].entry.Key)
		if left != right {
			return left < right
		}
		return scored[i].idx < scored[j].idx
	})

	result := make([]Entry, 0, len(scored))
	for _, item := range scored {
		result = append(result, item.entry)
	}
	return result
}

func matchScore(entry Entry, query string) (int, bool) {
	key := strings.ToLower(entry.Key)
	desc := strings.ToLower(entry.Meta.Description)

	if key == query {
		return 4000 + len(key), true
	}
	if strings.HasPrefix(key, query) {
		return 3000 + len(query)*10 - len(key), true
	}

	best := -1
	if score, ok := subsequenceScore(key, query); ok {
		best = 2000 + score
	}
	if score, ok := subsequenceScore(desc, query); ok {
		descScore := 1000 + score
		if descScore > best {
			best = descScore
		}
	}
	if best >= 0 {
		return best, true
	}
	return 0, false
}

func subsequenceScore(source, query string) (int, bool) {
	if query == "" {
		return 0, true
	}
	if source == "" {
		return 0, false
	}

	src := []rune(source)
	pos := -1
	total := 0
	streak := 0
	for _, qr := range query {
		found := false
		for i := pos + 1; i < len(src); i++ {
			if src[i] != qr {
				continue
			}
			gap := i - pos - 1
			if gap == 0 {
				streak++
				total += 18 + streak*6
			} else {
				streak = 0
				total += 12 - min(gap, 9)
			}
			if i == 0 {
				total += 20
			}
			pos = i
			found = true
			break
		}
		if !found {
			return 0, false
		}
	}
	return total - len(src), true
}
