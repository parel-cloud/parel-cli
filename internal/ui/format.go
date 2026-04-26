// Package ui contains the rendering primitives shared by the cobra commands.
package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"golang.org/x/term"
)

// PrintJSON writes v as indented JSON to stdout.
func PrintJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// PrintRawJSON writes already-marshalled JSON pretty-printed.
func PrintRawJSON(raw json.RawMessage) error {
	var buf []byte
	if len(raw) == 0 {
		return nil
	}
	var pretty any
	if err := json.Unmarshal(raw, &pretty); err != nil {
		// Not valid JSON — print as-is.
		_, err := os.Stdout.Write(raw)
		return err
	}
	out, err := json.MarshalIndent(pretty, "", "  ")
	if err != nil {
		return err
	}
	buf = append(buf, out...)
	buf = append(buf, '\n')
	_, err = os.Stdout.Write(buf)
	return err
}

// Table is a minimalist tabwriter wrapper. It is colour-free by default and
// safe to send to non-TTY targets (CI logs, redirected stdout).
type Table struct {
	w *tabwriter.Writer
}

func NewTable(out io.Writer) *Table {
	if out == nil {
		out = os.Stdout
	}
	return &Table{w: tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)}
}

func (t *Table) Headers(cols ...string) {
	for i, c := range cols {
		if i > 0 {
			fmt.Fprint(t.w, "\t")
		}
		fmt.Fprint(t.w, c)
	}
	fmt.Fprintln(t.w)
}

func (t *Table) Row(cols ...string) {
	for i, c := range cols {
		if i > 0 {
			fmt.Fprint(t.w, "\t")
		}
		fmt.Fprint(t.w, c)
	}
	fmt.Fprintln(t.w)
}

func (t *Table) Flush() error { return t.w.Flush() }

// IsTerminal reports whether stdout is a tty.
func IsTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}
