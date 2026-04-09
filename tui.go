package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

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
