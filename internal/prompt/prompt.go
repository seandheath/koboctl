// Package prompt provides testable stdio prompt helpers for interactive CLI commands.
//
// Create a Prompter with NewPrompter(os.Stdin, os.Stdout) in production, or inject
// a strings.NewReader / bytes.Buffer pair in tests.
package prompt

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Prompter wraps a reader/writer pair for interactive prompts.
type Prompter struct {
	in  *bufio.Reader
	out io.Writer
}

// NewPrompter creates a Prompter backed by the given reader and writer.
func NewPrompter(in io.Reader, out io.Writer) *Prompter {
	return &Prompter{in: bufio.NewReader(in), out: out}
}

// Bool asks a yes/no question and returns the boolean result.
// defaultYes controls what a bare Enter means and changes the displayed hint:
//   - true  → displays [Y/n]
//   - false → displays [y/N]
//
// Accepted inputs (case-insensitive): y, yes, n, no, or bare Enter.
// EOF is treated as "accept default".
func (p *Prompter) Bool(question string, defaultYes bool) (bool, error) {
	hint := "[y/N]"
	if defaultYes {
		hint = "[Y/n]"
	}
	fmt.Fprintf(p.out, "  %s %s: ", question, hint)

	line, err := p.readLine()
	if err != nil {
		return defaultYes, nil // EOF → use default
	}

	switch strings.ToLower(line) {
	case "", " ":
		return defaultYes, nil
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	default:
		fmt.Fprintf(p.out, "  Please enter y or n.\n")
		return p.Bool(question, defaultYes)
	}
}

// String asks for a string value, showing the current default in brackets.
// A bare Enter returns defaultVal unchanged.
// EOF is treated as "accept default".
func (p *Prompter) String(question, defaultVal string) (string, error) {
	fmt.Fprintf(p.out, "  %s [%s]: ", question, defaultVal)

	line, err := p.readLine()
	if err != nil || line == "" {
		return defaultVal, nil
	}
	return line, nil
}

// Choice asks the user to pick one value from a fixed set, showing the default.
// A bare Enter returns defaultVal. Invalid input re-prompts with a hint listing
// the valid choices. EOF is treated as "accept default".
func (p *Prompter) Choice(question string, choices []string, defaultVal string) (string, error) {
	fmt.Fprintf(p.out, "  %s (%s) [%s]: ", question, strings.Join(choices, "/"), defaultVal)

	line, err := p.readLine()
	if err != nil || line == "" {
		return defaultVal, nil
	}

	for _, c := range choices {
		if strings.EqualFold(line, c) {
			return c, nil
		}
	}

	fmt.Fprintf(p.out, "  Invalid choice %q. Choose one of: %s\n", line, strings.Join(choices, ", "))
	return p.Choice(question, choices, defaultVal)
}

// StringList asks for a comma-separated list of string values.
// A bare Enter returns defaultVals unchanged. EOF is treated as "accept default".
// Leading/trailing whitespace is trimmed from each item; empty items are dropped.
func (p *Prompter) StringList(question string, defaultVals []string) ([]string, error) {
	fmt.Fprintf(p.out, "  %s [%s]: ", question, strings.Join(defaultVals, ","))

	line, err := p.readLine()
	if err != nil || line == "" {
		return defaultVals, nil
	}

	var result []string
	for _, item := range strings.Split(line, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
	}
	if len(result) == 0 {
		return defaultVals, nil
	}
	return result, nil
}

// readLine reads one line from the input, stripping the trailing newline.
// Returns io.EOF if the reader is exhausted.
func (p *Prompter) readLine() (string, error) {
	line, err := p.in.ReadString('\n')
	line = strings.TrimRight(line, "\r\n")
	if err == io.EOF {
		if line != "" {
			return line, nil // last line without newline
		}
		return "", io.EOF
	}
	return line, err
}
