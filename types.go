package main

import "github.com/charmbracelet/bubbles/textinput"

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
