# yanker

`yanker` is a small terminal clipboard helper for named values and secrets.

It opens straight into a Bubble Tea TUI for browsing, filtering, previewing, and copying entries, and it also supports quick CLI commands for add/get/pick flows without opening the full interface.

## Features

- Single-binary, single-file app structure
- JSON-backed storage
- In-memory fuzzy filtering across key and description
- Secret-aware previews that never render full secret values in the TUI
- Clipboard copy from both TUI and CLI
- In-app add, edit, and delete flows
- Quick CLI commands for direct add/get/pick usage
- Atomic saves with restrictive file permissions

## Entry Format

Entries are stored as JSON objects like this:

```json
[
  {
    "key": "gh-token",
    "value": "ghp_...",
    "meta": {
      "kind": "secret",
      "description": "GitHub token"
    }
  }
]
```

Rules:

- `key` must be unique
- `kind` controls preview behavior
- `description` is used for readability and filtering

Supported kinds:

- `plain`
- `secret`

## Install / Build

```bash
make build
```

This writes the binary to:

- `bin/yanker`

Run it with:

```bash
./bin/yanker
```

Install it into your PATH for user-local usage:

```bash
make install-user
```

By default this installs to:

- `~/.local/bin/yanker`

If `~/.local/bin` is already on your `PATH`, you can run `yanker` from anywhere.

You can also install to a custom prefix:

```bash
make install PREFIX=/usr/local
```

Useful commands:

- `make help`
- `make build`
- `make run`
- `make fmt`
- `make test`
- `make clean`
- `make uninstall`

## Commands

### Open the TUI

```bash
yanker
```

### Add an entry

```bash
yanker add <key> <value> [-kind plain|secret] [-desc "text"]
```

Examples:

```bash
yanker add gh-token ghp_xxx -kind secret -desc "GitHub token"
yanker add api-url https://example.com -desc "Production API"
```

Notes:

- `plain` is the default kind
- flags can be placed after the positional arguments, as shown above

### Get a value

```bash
yanker get <key>
```

This prints the raw value to stdout.

### Pick the best match

```bash
yanker pick [query]
```

Behavior:

- finds the best matching entry
- copies its value to the clipboard
- prints the selected key

Example:

```bash
yanker pick git
```

## TUI Usage

The TUI has three lightweight modes:

- list mode
- add/edit form mode
- delete confirmation mode

### List mode

When `yanker` starts, it opens in list mode with:

- filter input at the top
- results list
- preview/details pane
- footer keybind hints

Keybindings in list mode:

- `/` enter search mode
- `j` / `k` move selection
- `Up` / `Down` move selection
- `Enter` copy selected value
- `y` copy selected value
- `a` add a new entry
- `e` edit the selected entry
- `d` delete the selected entry
- `q` quit

### Search mode

Search mode is separate from command mode so single-key actions like `a`, `e`, and `d` do not conflict with typing.

How it works:

- press `/` to focus the filter
- type to fuzzy filter entries
- `Up` / `Down` moves while searching
- `Enter` copies the current selection
- `Esc` clears the filter and exits search mode

Typing any regular character from list mode also starts searching immediately.

### Add / Edit mode

Fields:

- key
- value
- kind
- description

Behavior:

- `Enter` always submits the form
- `Tab` / `Shift+Tab` moves between fields
- `Esc` cancels and returns to list mode
- validation errors are shown inline
- focus jumps to the first field with an error

Validation:

- key is required
- value is required
- key must be unique
- kind must be `plain` or `secret`

Editing secrets:

- secret values are never shown in the TUI
- when editing a secret, leave the value blank to keep the existing value
- enter a new value only if you want to replace it

### Delete confirmation

Keybindings:

- `y` or `Enter` confirms delete
- `n` or `Esc` cancels

## Preview Behavior

For plain values:

- the preview pane shows the value

For secret values:

- the TUI only shows a masked preview
- full secret values are still available for clipboard copy and `get`

## Filtering and Ranking

Filtering searches across:

- key
- description

Ranking preference:

1. exact key match
2. key prefix match
3. fuzzy subsequence score
4. alphabetical fallback

## Storage

Default storage path:

- `$XDG_DATA_HOME/yanker/entries.json` when `XDG_DATA_HOME` is set
- otherwise `~/.local/share/yanker/entries.json`

Behavior:

- data is loaded once at startup
- entries stay in memory while the app runs
- add/edit/delete writes back immediately
- writes are atomic
- files are created with restrictive permissions

## Current Notes

- the TUI masks secrets in previews but `yanker get` prints the raw value by design
- `pick` copies the best match and prints the matched key
- the app currently keeps everything in `main.go` to stay compact
