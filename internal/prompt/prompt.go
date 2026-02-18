// Package prompt provides simple interactive prompts for terminal input.
package prompt

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

// scanLine reads a single line from stdin byte-by-byte with no buffering.
// This avoids any shared state that could be corrupted by term.ReadPassword
// reading from the raw file descriptor.
func scanLine() (string, bool) {
	var buf []byte
	b := make([]byte, 1)
	for {
		n, err := os.Stdin.Read(b)
		if err != nil || n == 0 {
			if len(buf) > 0 {
				return strings.TrimSpace(string(buf)), true
			}
			return "", false
		}
		if b[0] == '\n' {
			return strings.TrimSpace(string(buf)), true
		}
		if b[0] != '\r' {
			buf = append(buf, b[0])
		}
	}
}

// Confirm asks a yes/no question and returns true for yes.
func Confirm(question string) bool {
	fmt.Fprintf(os.Stderr, "%s (yes/no): ", question)
	answer, ok := scanLine()
	if !ok {
		return false
	}
	answer = strings.ToLower(answer)
	return answer == "yes" || answer == "y"
}

// SelectMulti displays a numbered list and lets the user pick items.
// Accepts "all" or comma-separated indices (1-based). Returns selected items.
func SelectMulti(label string, items []string) []string {
	fmt.Fprintf(os.Stderr, "%s:\n", label)
	for i, item := range items {
		fmt.Fprintf(os.Stderr, "  [%d] %s\n", i+1, item)
	}
	fmt.Fprint(os.Stderr, "\nEnter numbers (comma-separated, e.g. 1,3,5) or 'all': ")
	input, ok := scanLine()
	if !ok {
		return nil
	}
	if strings.EqualFold(input, "all") {
		return items
	}

	var selected []string
	for _, part := range strings.Split(input, ",") {
		idx, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || idx < 1 || idx > len(items) {
			continue
		}
		selected = append(selected, items[idx-1])
	}
	return selected
}

// Select displays a numbered list and lets the user pick one item.
// Returns the selected item string, or "" if input is invalid.
func Select(label string, items []string) string {
	return SelectWithOther(label, items, false)
}

// SelectWithOther displays a numbered list with an optional "Other" entry.
// If allowOther is true, an extra option lets the user type a custom value.
// The user can enter a number, or type text directly:
//   - With allowOther: non-numeric text is accepted as a freeform value.
//   - Without allowOther: text is matched (case-insensitive) against the first
//     word of each item (e.g. typing "local" matches "local - Docker Compose...").
func SelectWithOther(label string, items []string, allowOther bool) string {
	maxAttempts := 3
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt == 0 {
			// Show the menu only on the first attempt (or after invalid input)
			printMenu(label, items, allowOther)
		}
		fmt.Fprint(os.Stderr, "Enter choice: ")
		input, ok := scanLine()
		if !ok {
			return ""
		}
		if input == "" {
			fmt.Fprintln(os.Stderr, "  Invalid choice, try again.")
			printMenu(label, items, allowOther)
			continue
		}

		// Try numeric selection first.
		if idx, err := strconv.Atoi(input); err == nil {
			if idx >= 1 && idx <= len(items) {
				return items[idx-1]
			}
			if allowOther && idx == len(items)+1 {
				return ReadLine("Enter value")
			}
			fmt.Fprintln(os.Stderr, "  Invalid choice, try again.")
			printMenu(label, items, allowOther)
			continue
		}

		// Non-numeric input: if allowOther, accept as freeform value.
		if allowOther {
			return input
		}

		// Try matching against the first word of each item (case-insensitive).
		inputLower := strings.ToLower(input)
		var match string
		matches := 0
		for _, item := range items {
			firstWord := strings.ToLower(strings.SplitN(item, " ", 2)[0])
			if firstWord == inputLower {
				match = item
				matches++
			}
		}
		if matches == 1 {
			return match
		}

		fmt.Fprintln(os.Stderr, "  Invalid choice, try again.")
		printMenu(label, items, allowOther)
	}
	return ""
}

func printMenu(label string, items []string, allowOther bool) {
	fmt.Fprintf(os.Stderr, "%s:\n", label)
	for i, item := range items {
		fmt.Fprintf(os.Stderr, "  [%d] %s\n", i+1, item)
	}
	if allowOther {
		fmt.Fprintf(os.Stderr, "  [%d] Other (enter manually)\n", len(items)+1)
	}
}

// ReadLine prompts for a single line of text input.
func ReadLine(label string) string {
	fmt.Fprintf(os.Stderr, "%s: ", label)
	line, _ := scanLine()
	return line
}

// ReadSecret reads input with terminal echo disabled (for tokens/passwords).
// Falls back to plain ReadLine if stdin is not a terminal.
func ReadSecret(label string) string {
	fmt.Fprintf(os.Stderr, "%s: ", label)
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		raw, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err == nil {
			return strings.TrimSpace(string(raw))
		}
	}
	line, _ := scanLine()
	return line
}

// SelectMultiWithDefaults is like SelectMulti but pre-selects default indices.
// defaultIdxs uses 1-based numbering. Pressing Enter accepts the defaults.
func SelectMultiWithDefaults(label string, items []string, defaultIdxs []int) []string {
	var defStrs []string
	for _, i := range defaultIdxs {
		defStrs = append(defStrs, strconv.Itoa(i))
	}
	defaultDisplay := strings.Join(defStrs, ",")

	fmt.Fprintf(os.Stderr, "%s:\n", label)
	for i, item := range items {
		marker := "   "
		for _, di := range defaultIdxs {
			if di == i+1 {
				marker = " * "
				break
			}
		}
		fmt.Fprintf(os.Stderr, " %s[%d] %s\n", marker, i+1, item)
	}
	fmt.Fprintf(os.Stderr, "   (* = default)\n")
	fmt.Fprintf(os.Stderr, "\nEnter numbers (comma-separated) or 'all' [Enter for default (%s)]: ", defaultDisplay)

	input, ok := scanLine()
	if !ok {
		return applyDefaults(items, defaultIdxs)
	}
	if input == "" {
		return applyDefaults(items, defaultIdxs)
	}
	if strings.EqualFold(input, "all") {
		return items
	}
	var selected []string
	for _, part := range strings.Split(input, ",") {
		idx, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || idx < 1 || idx > len(items) {
			continue
		}
		selected = append(selected, items[idx-1])
	}
	return selected
}

func applyDefaults(items []string, idxs []int) []string {
	var out []string
	for _, i := range idxs {
		if i >= 1 && i <= len(items) {
			out = append(out, items[i-1])
		}
	}
	return out
}
