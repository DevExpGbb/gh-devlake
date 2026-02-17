// Package prompt provides simple interactive prompts using bufio.Scanner.
package prompt

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

var scanner = bufio.NewScanner(os.Stdin)

// Confirm asks a yes/no question and returns true for yes.
func Confirm(question string) bool {
	fmt.Fprintf(os.Stderr, "%s (yes/no): ", question)
	if !scanner.Scan() {
		return false
	}
	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
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
	if !scanner.Scan() {
		return nil
	}
	input := strings.TrimSpace(scanner.Text())
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
	fmt.Fprintf(os.Stderr, "%s:\n", label)
	for i, item := range items {
		fmt.Fprintf(os.Stderr, "  [%d] %s\n", i+1, item)
	}
	fmt.Fprint(os.Stderr, "\nEnter number: ")
	if !scanner.Scan() {
		return ""
	}
	idx, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
	if err != nil || idx < 1 || idx > len(items) {
		return ""
	}
	return items[idx-1]
}

// ReadLine prompts for a single line of text input.
func ReadLine(label string) string {
	fmt.Fprintf(os.Stderr, "%s: ", label)
	if !scanner.Scan() {
		return ""
	}
	return strings.TrimSpace(scanner.Text())
}
