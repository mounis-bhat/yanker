package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Entry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Meta  Meta   `json:"meta"`
}

type Meta struct {
	Kind        string `json:"kind,omitempty"`
	Description string `json:"description,omitempty"`
}

type mode int

const (
	modeList mode = iota
	modeForm
	modeConfirmDelete
)

const (
	kindPlain  = "plain"
	kindSecret = "secret"
)

const (
	fieldKey = iota
	fieldValue
	fieldKind
	fieldDescription
	fieldCount
)

type scoredEntry struct {
	entry Entry
	score int
	idx   int
}

type formState struct {
	editing      bool
	originalKey  string
	secretEdit   bool
	inputs       []textinput.Model
	focus        int
	errorMessage string
	helpMessage  string
	submitLabel  string
	heading      string
}

type model struct {
	storePath      string
	entries        []Entry
	filtered       []Entry
	filter         textinput.Model
	filterActive   bool
	cursor         int
	mode           mode
	form           formState
	width          int
	height         int
	status         string
	statusErr      bool
	confirmMessage string
}

type errMsg struct {
	err error
}

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

func runTUI(storePath string, entries []Entry) error {
	filter := textinput.New()
	filter.Placeholder = "Press / to search"
	filter.CharLimit = 256
	filter.Prompt = "Filter: "
	filter.Blur()

	m := model{
		storePath: storePath,
		entries:   append([]Entry(nil), entries...),
		filter:    filter,
		mode:      modeList,
	}
	m.refreshFiltered()

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

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
	defer os.Remove(tmpPath)

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmpPath, path)
}

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
	q := []rune(query)
	pos := -1
	total := 0
	streak := 0
	for _, qr := range q {
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func copyToClipboard(value string) error {
	return clipboard.WriteAll(value)
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case errMsg:
		m.setError(msg.err)
		return m, nil
	case tea.KeyMsg:
		switch m.mode {
		case modeList:
			return m.updateList(msg)
		case modeForm:
			return m.updateForm(msg)
		case modeConfirmDelete:
			return m.updateDeleteConfirm(msg)
		}
	}

	if m.mode == modeList {
		var cmd tea.Cmd
		m.filter, cmd = m.filter.Update(msg)
		m.refreshFiltered()
		return m, cmd
	}

	return m, nil
}

func (m model) View() string {
	styles := newStyles()
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	switch m.mode {
	case modeForm:
		return m.viewForm(styles)
	case modeConfirmDelete:
		return m.viewConfirmDelete(styles)
	default:
		return m.viewList(styles)
	}
}

func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.filterActive {
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.filter.SetValue("")
			m.refreshFiltered()
			m.filterActive = false
			m.filter.Blur()
			m.setStatus("Search cleared")
			return m, nil
		case "up":
			m.moveCursor(-1)
			return m, nil
		case "down":
			m.moveCursor(1)
			return m, nil
		case "enter":
			entry, ok := m.selectedEntry()
			if !ok {
				return m, nil
			}
			if err := copyToClipboard(entry.Value); err != nil {
				m.setError(err)
				return m, nil
			}
			m.setStatus(fmt.Sprintf("Copied %s", entry.Key))
			return m, nil
		}

		var cmd tea.Cmd
		m.filter, cmd = m.filter.Update(msg)
		m.refreshFiltered()
		return m, cmd
	}

	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up", "k":
		m.moveCursor(-1)
		return m, nil
	case "down", "j":
		m.moveCursor(1)
		return m, nil
	case "enter", "y":
		entry, ok := m.selectedEntry()
		if !ok {
			return m, nil
		}
		if err := copyToClipboard(entry.Value); err != nil {
			m.setError(err)
			return m, nil
		}
		m.setStatus(fmt.Sprintf("Copied %s", entry.Key))
		return m, nil
	case "a":
		m.beginAdd()
		return m, textinput.Blink
	case "e":
		if entry, ok := m.selectedEntry(); ok {
			m.beginEdit(entry)
			return m, textinput.Blink
		}
		return m, nil
	case "d":
		if entry, ok := m.selectedEntry(); ok {
			m.mode = modeConfirmDelete
			m.confirmMessage = fmt.Sprintf("Delete %s?", entry.Key)
		}
		return m, nil
	case "/":
		m.filterActive = true
		m.filter.Focus()
		m.status = ""
		m.statusErr = false
		return m, textinput.Blink
	}

	if msg.Type == tea.KeyRunes {
		m.filterActive = true
		m.filter.Focus()
		var cmd tea.Cmd
		m.filter, cmd = m.filter.Update(msg)
		m.refreshFiltered()
		return m, cmd
	}

	return m, nil
}

func (m model) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeList
		m.form = formState{}
		m.filterActive = false
		m.filter.Blur()
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "shift+tab", "up":
		m.focusForm(-1)
		return m, nil
	case "tab", "down":
		m.focusForm(1)
		return m, nil
	case "enter":
		if err := m.submitForm(); err != nil {
			m.form.errorMessage = err.Error()
			return m, nil
		}
		return m, nil
	}

	var cmd tea.Cmd
	input := m.form.inputs[m.form.focus]
	input, cmd = input.Update(msg)
	m.form.inputs[m.form.focus] = input
	m.form.errorMessage = ""
	return m, cmd
}

func (m model) updateDeleteConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "n":
		m.mode = modeList
		m.confirmMessage = ""
		return m, nil
	case "y", "enter":
		entry, ok := m.selectedEntry()
		if !ok {
			m.mode = modeList
			m.confirmMessage = ""
			return m, nil
		}
		updated := make([]Entry, 0, len(m.entries)-1)
		for _, existing := range m.entries {
			if existing.Key != entry.Key {
				updated = append(updated, existing)
			}
		}
		if err := saveEntries(m.storePath, updated); err != nil {
			m.setError(err)
			m.mode = modeList
			m.confirmMessage = ""
			return m, nil
		}
		m.entries = updated
		m.mode = modeList
		m.confirmMessage = ""
		m.refreshFiltered()
		m.setStatus(fmt.Sprintf("Deleted %s", entry.Key))
		return m, nil
	}
	return m, nil
}

func (m *model) submitForm() error {
	entry := Entry{
		Key:   strings.TrimSpace(m.form.inputs[fieldKey].Value()),
		Value: m.form.inputs[fieldValue].Value(),
		Meta: Meta{
			Kind:        normalizeKind(m.form.inputs[fieldKind].Value()),
			Description: strings.TrimSpace(m.form.inputs[fieldDescription].Value()),
		},
	}

	if m.form.editing && m.form.secretEdit && entry.Value == "" {
		if current, ok := findEntry(m.entries, m.form.originalKey); ok {
			entry.Value = current.Value
		}
	}

	if field, err := validateEntryField(entry, m.entries, m.form.originalKey); err != nil {
		m.focusFormField(field)
		return err
	}

	updated := append([]Entry(nil), m.entries...)
	if m.form.editing {
		for i := range updated {
			if updated[i].Key == m.form.originalKey {
				updated[i] = entry
				sortEntries(updated)
				if err := saveEntries(m.storePath, updated); err != nil {
					return err
				}
				m.entries = updated
				m.mode = modeList
				m.form = formState{}
				m.filterActive = false
				m.filter.Blur()
				m.refreshFiltered()
				m.setStatus(fmt.Sprintf("Updated %s", entry.Key))
				return nil
			}
		}
		return fmt.Errorf("entry %q no longer exists", m.form.originalKey)
	}

	updated = append(updated, entry)
	sortEntries(updated)
	if err := saveEntries(m.storePath, updated); err != nil {
		return err
	}
	m.entries = updated
	m.mode = modeList
	m.form = formState{}
	m.filterActive = false
	m.filter.Blur()
	m.refreshFiltered()
	m.setStatus(fmt.Sprintf("Added %s", entry.Key))
	return nil
}

func (m *model) focusFormField(field int) {
	if field < 0 || field >= len(m.form.inputs) {
		return
	}
	m.form.inputs[m.form.focus].Blur()
	m.form.focus = field
	m.form.inputs[m.form.focus].Focus()
}

func (m *model) beginAdd() {
	m.mode = modeForm
	m.form = newFormState(false, Entry{})
	for i := range m.form.inputs {
		m.form.inputs[i].Blur()
	}
	m.form.inputs[0].Focus()
}

func (m *model) beginEdit(entry Entry) {
	m.mode = modeForm
	m.form = newFormState(true, entry)
	for i := range m.form.inputs {
		m.form.inputs[i].Blur()
	}
	m.form.inputs[0].Focus()
}

func newFormState(editing bool, entry Entry) formState {
	inputs := make([]textinput.Model, fieldCount)
	placeholders := []string{"key", "value", "plain or secret", "description"}
	values := []string{entry.Key, entry.Value, normalizeKind(entry.Meta.Kind), entry.Meta.Description}

	for i := range inputs {
		inputs[i] = textinput.New()
		inputs[i].Prompt = ""
		inputs[i].CharLimit = 4096
		inputs[i].Width = 48
		inputs[i].Placeholder = placeholders[i]
		inputs[i].SetValue(values[i])
	}

	state := formState{
		editing:     editing,
		originalKey: entry.Key,
		inputs:      inputs,
		focus:       0,
		submitLabel: "Add entry",
		heading:     "Add Entry",
		helpMessage: "Enter submits. Tab and shift+tab move between fields.",
	}

	if editing {
		state.submitLabel = "Save changes"
		state.heading = "Edit Entry"
		if normalizeKind(entry.Meta.Kind) == kindSecret {
			state.secretEdit = true
			state.inputs[fieldValue].SetValue("")
			state.inputs[fieldValue].Placeholder = "leave blank to keep existing secret"
			state.helpMessage = "Secret values stay masked. Enter a new value only if you want to replace it."
		}
	}

	return state
}

func (m *model) focusForm(delta int) {
	m.form.inputs[m.form.focus].Blur()
	m.form.focus = (m.form.focus + delta + len(m.form.inputs)) % len(m.form.inputs)
	m.form.inputs[m.form.focus].Focus()
}

func (m *model) moveCursor(delta int) {
	if len(m.filtered) == 0 {
		m.cursor = 0
		return
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
}

func (m *model) refreshFiltered() {
	m.filtered = filterEntries(m.entries, m.filter.Value())
	if len(m.filtered) == 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m model) selectedEntry() (Entry, bool) {
	if len(m.filtered) == 0 || m.cursor < 0 || m.cursor >= len(m.filtered) {
		return Entry{}, false
	}
	return m.filtered[m.cursor], true
}

func (m *model) setStatus(message string) {
	m.status = message
	m.statusErr = false
	if m.mode == modeList && m.filterActive {
		m.filter.Focus()
	}
}

func (m *model) setError(err error) {
	m.status = err.Error()
	m.statusErr = true
	if m.mode == modeList && m.filterActive {
		m.filter.Focus()
	}
}

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

func (m model) viewList(s styles) string {
	title := s.title.Render("Yanker")
	filter := m.filter.View()

	listWidth := max(32, m.width/2)
	previewWidth := max(28, m.width-listWidth-1)
	contentHeight := max(10, m.height-6)

	listPanel := s.panel.Width(listWidth - 2).Height(contentHeight).Render(m.renderList(s, contentHeight-2, listWidth-4))
	previewPanel := s.selectedPanel.Width(previewWidth - 2).Height(contentHeight).Render(m.renderPreview(s, previewWidth-4, contentHeight-2))

	content := ""
	if m.width >= 100 {
		content = lipgloss.JoinHorizontal(lipgloss.Top, listPanel, previewPanel)
	} else {
		fullWidth := m.width - 2
		listPanel = s.panel.Width(fullWidth).Height(contentHeight / 2).Render(m.renderList(s, contentHeight/2-2, fullWidth-4))
		previewPanel = s.selectedPanel.Width(fullWidth).Height(contentHeight - (contentHeight / 2)).Render(m.renderPreview(s, fullWidth-4, contentHeight-(contentHeight/2)-2))
		content = lipgloss.JoinVertical(lipgloss.Left, listPanel, previewPanel)
	}

	status := m.status
	if status == "" {
		status = fmt.Sprintf("%d entries", len(m.entries))
	}
	statusStyle := s.status
	if m.statusErr {
		statusStyle = s.errorStatus
	}

	footer := s.footer.Render("/ search  j/k, arrows move  enter/y copy  a add  e edit  d delete  q quit")
	if m.filterActive {
		footer = s.footer.Render("type to search  up/down move  enter copy  esc clear search")
	}
	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		filter,
		content,
		statusStyle.Render(status),
		footer,
	)
}

func (m model) renderList(s styles, height, width int) string {
	if len(m.filtered) == 0 {
		return s.muted.Render("No entries match the current filter.")
	}

	rows := make([]string, 0, min(height, len(m.filtered)))
	start := 0
	if m.cursor >= height {
		start = m.cursor - height + 1
	}
	end := min(len(m.filtered), start+height)
	for i := start; i < end; i++ {
		entry := m.filtered[i]
		marker := "  "
		style := s.row
		if i == m.cursor {
			marker = s.marker.Render("▌ ")
			style = s.selectedRow
		}

		meta := entry.Meta.Description
		if meta == "" {
			meta = maskPreview(entry)
		}
		line := style.Render(truncate(fmt.Sprintf("%s%s", entry.Key, renderBadges(s, entry)), max(0, width-2)))
		detail := s.muted.Render(truncate(meta, max(0, width-2)))
		rows = append(rows, marker+line)
		rows = append(rows, "  "+detail)
	}
	return strings.Join(rows, "\n")
}

func renderBadges(s styles, entry Entry) string {
	if normalizeKind(entry.Meta.Kind) != kindSecret {
		return ""
	}
	return " " + s.badge.Render("secret")
}

func (m model) renderPreview(s styles, width, height int) string {
	entry, ok := m.selectedEntry()
	if !ok {
		return s.muted.Render("Select an entry to preview it.")
	}

	lines := []string{
		s.label.Render("Key") + "\n" + entry.Key,
		"",
		s.label.Render("Kind") + "\n" + normalizeKind(entry.Meta.Kind),
	}
	if entry.Meta.Description != "" {
		lines = append(lines, "", s.label.Render("Description")+"\n"+entry.Meta.Description)
	}
	lines = append(lines, "", s.label.Render("Preview")+"\n"+maskPreview(entry))

	content := wrapLines(lines, width)
	if lineCount(content) > height {
		return truncateLines(content, height)
	}
	return content
}

func (m model) viewForm(s styles) string {
	fields := []string{"Key", "Value", "Kind", "Description"}
	lines := []string{s.title.Render(m.form.heading), s.muted.Render(m.form.helpMessage), ""}
	for i, label := range fields {
		field := m.form.inputs[i].View()
		if i == fieldValue && m.form.secretEdit {
			field = m.form.inputs[i].View()
		}
		lines = append(lines, s.label.Render(label), s.inputBox.Width(max(30, min(72, m.width-10))).Render(field), "")
	}
	if m.form.errorMessage != "" {
		lines = append(lines, s.errorText.Render(m.form.errorMessage), "")
	}
	lines = append(lines, s.button.Render(m.form.submitLabel), s.footer.Render("tab/shift+tab move  enter next/submit  esc cancel"))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, s.panel.Render(strings.Join(lines, "\n")))
}

func (m model) viewConfirmDelete(s styles) string {
	content := []string{
		s.title.Render("Confirm Delete"),
		"",
		m.confirmMessage,
		"",
		s.footer.Render("y/enter confirm  n/esc cancel"),
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, s.selectedPanel.Render(strings.Join(content, "\n")))
}

func maskPreview(entry Entry) string {
	if normalizeKind(entry.Meta.Kind) != kindSecret {
		return entry.Value
	}
	runes := []rune(entry.Value)
	if len(runes) == 0 {
		return "(empty secret)"
	}
	if len(runes) <= 4 {
		return strings.Repeat("*", len(runes))
	}
	prefixLen := min(4, len(runes))
	suffixLen := min(4, len(runes)-prefixLen)
	maskedLen := len(runes) - prefixLen - suffixLen
	if maskedLen < 4 {
		maskedLen = 4
	}
	return string(runes[:prefixLen]) + strings.Repeat("*", maskedLen) + string(runes[len(runes)-suffixLen:])
}

func truncate(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if utf8.RuneCountInString(value) <= width {
		return value
	}
	runes := []rune(value)
	if width <= 1 {
		return string(runes[:width])
	}
	return string(runes[:width-1]) + "…"
}

func wrapLines(lines []string, width int) string {
	wrapped := make([]string, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			wrapped = append(wrapped, "")
			continue
		}
		for _, part := range strings.Split(line, "\n") {
			wrapped = append(wrapped, lipgloss.NewStyle().Width(width).Render(part))
		}
	}
	return strings.Join(wrapped, "\n")
}

func lineCount(value string) int {
	return len(strings.Split(value, "\n"))
}

func truncateLines(value string, limit int) string {
	lines := strings.Split(value, "\n")
	if len(lines) <= limit {
		return value
	}
	return strings.Join(lines[:limit], "\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
