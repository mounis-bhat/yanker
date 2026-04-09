package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
)

func printUsage() {
	fmt.Println("yanker")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  yanker                         Open the TUI")
	fmt.Println("  yanker add <key> <value> [-kind plain|secret] [-desc text]")
	fmt.Println("  yanker get <key>               Print the raw value")
	fmt.Println("  yanker pick [query]            Copy best match and print the key")
}

func runAdd(storePath string, entries []Entry, args []string) error {
	args = normalizeAddArgs(args)
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	kind := fs.String("kind", kindPlain, "entry kind")
	desc := fs.String("desc", "", "entry description")
	if err := fs.Parse(args); err != nil {
		return err
	}
	positional := fs.Args()
	if len(positional) != 2 {
		return errors.New("usage: yanker add <key> <value> [-kind plain|secret] [-desc text]")
	}

	entry := Entry{
		Key:   strings.TrimSpace(positional[0]),
		Value: positional[1],
		Meta: Meta{
			Kind:        normalizeKind(*kind),
			Description: strings.TrimSpace(*desc),
		},
	}
	if err := validateEntry(entry, entries, ""); err != nil {
		return err
	}

	entries = append(entries, entry)
	sortEntries(entries)
	if err := saveEntries(storePath, entries); err != nil {
		return err
	}

	fmt.Printf("added %s\n", entry.Key)
	return nil
}

func normalizeAddArgs(args []string) []string {
	flags := make([]string, 0, len(args))
	positional := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-kind", "-desc":
			flags = append(flags, arg)
			if i+1 < len(args) {
				flags = append(flags, args[i+1])
				i++
			}
		default:
			flags = append(flags, arg)
			if strings.HasPrefix(arg, "-") {
				continue
			}
			flags = flags[:len(flags)-1]
			positional = append(positional, arg)
		}
	}

	return append(flags, positional...)
}

func runGet(entries []Entry, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: yanker get <key>")
	}

	entry, ok := findEntry(entries, args[0])
	if !ok {
		return fmt.Errorf("no entry found for %q", args[0])
	}

	fmt.Print(entry.Value)
	return nil
}

func runPick(entries []Entry, args []string) error {
	query := ""
	if len(args) > 1 {
		return errors.New("usage: yanker pick [query]")
	}
	if len(args) == 1 {
		query = args[0]
	}

	filtered := filterEntries(entries, query)
	if len(filtered) == 0 {
		return fmt.Errorf("no entries match %q", query)
	}

	entry := filtered[0]
	if err := copyToClipboard(entry.Value); err != nil {
		return err
	}

	fmt.Println(entry.Key)
	return nil
}
