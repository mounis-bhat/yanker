package main

import "github.com/atotto/clipboard"

func copyToClipboard(value string) error {
	return clipboard.WriteAll(value)
}
