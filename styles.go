package main

import "github.com/charmbracelet/lipgloss"

type styles struct {
	title         lipgloss.Style
	panel         lipgloss.Style
	selectedPanel lipgloss.Style
	muted         lipgloss.Style
	errorText     lipgloss.Style
	selectedRow   lipgloss.Style
	row           lipgloss.Style
	badge         lipgloss.Style
	footer        lipgloss.Style
	status        lipgloss.Style
	errorStatus   lipgloss.Style
	label         lipgloss.Style
	inputBox      lipgloss.Style
	button        lipgloss.Style
	marker        lipgloss.Style
}

func newStyles() styles {
	border := lipgloss.AdaptiveColor{Light: "#A3A3A3", Dark: "#5F5F5F"}
	accent := lipgloss.AdaptiveColor{Light: "#1D4ED8", Dark: "#93C5FD"}
	muted := lipgloss.AdaptiveColor{Light: "#666666", Dark: "#9CA3AF"}
	soft := lipgloss.AdaptiveColor{Light: "#E5E7EB", Dark: "#1F2937"}
	errorColor := lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#FCA5A5"}

	return styles{
		title:         lipgloss.NewStyle().Bold(true),
		panel:         lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).Padding(0, 1),
		selectedPanel: lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(accent).Padding(0, 1),
		muted:         lipgloss.NewStyle().Foreground(muted),
		errorText:     lipgloss.NewStyle().Foreground(errorColor).Bold(true),
		selectedRow:   lipgloss.NewStyle().Bold(true),
		row:           lipgloss.NewStyle(),
		badge:         lipgloss.NewStyle().Foreground(accent).Background(soft).Padding(0, 1),
		footer:        lipgloss.NewStyle().Foreground(muted),
		status:        lipgloss.NewStyle().Foreground(accent).Bold(true),
		errorStatus:   lipgloss.NewStyle().Foreground(errorColor).Bold(true),
		label:         lipgloss.NewStyle().Bold(true),
		inputBox:      lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(border).Padding(0, 1),
		button:        lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(accent).Padding(0, 1).Bold(true),
		marker:        lipgloss.NewStyle().Foreground(accent).Bold(true),
	}
}
