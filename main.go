package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	storePath, err := storagePath()
	if err != nil {
		return err
	}

	entries, err := loadEntries(storePath)
	if err != nil {
		return err
	}

	if len(args) == 0 {
		return runTUI(storePath, entries)
	}

	switch args[0] {
	case "add":
		return runAdd(storePath, entries, args[1:])
	case "get":
		return runGet(entries, args[1:])
	case "pick":
		return runPick(entries, args[1:])
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}
