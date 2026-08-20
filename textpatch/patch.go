// Package textpatch applies pathless Codex-style patches to text.
package textpatch

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

var (
	ErrInvalidPatch = errors.New("textpatch: invalid patch")
	ErrNoMatch      = errors.New("textpatch: expected text not found")
	ErrAmbiguous    = errors.New("textpatch: expected text is ambiguous")
)

const (
	beginMarker = "*** Begin Patch"
	endMarker   = "*** End Patch"
)

type hunk struct {
	context    string
	hasContext bool
	oldLines   []string
	newLines   []string
}

// Apply applies every patch hunk to original. On failure it returns original
// unchanged.
func Apply(original, patch string) (string, error) {
	if !utf8.ValidString(original) {
		return original, fmt.Errorf("%w: original contents are not UTF-8", ErrInvalidPatch)
	}
	if !utf8.ValidString(patch) {
		return original, fmt.Errorf("%w: patch is not UTF-8", ErrInvalidPatch)
	}

	hunks, err := parse(patch)
	if err != nil {
		return original, err
	}

	trailingNewline := strings.HasSuffix(original, "\n")
	lines := splitLines(original)
	searchFrom := 0
	for index, hunk := range hunks {
		if hunk.hasContext {
			match, err := uniqueMatch(lines, []string{hunk.context}, searchFrom)
			if err != nil {
				return original, fmt.Errorf("hunk %d context %q: %w", index+1, hunk.context, err)
			}
			searchFrom = match + 1
		}

		match, err := uniqueMatch(lines, hunk.oldLines, searchFrom)
		if err != nil {
			return original, fmt.Errorf("hunk %d: %w", index+1, err)
		}
		lines = replace(lines, match, len(hunk.oldLines), hunk.newLines)
		searchFrom = match + len(hunk.newLines)
	}

	updated := strings.Join(lines, "\n")
	if trailingNewline {
		updated += "\n"
	}
	return updated, nil
}

func parse(patch string) ([]hunk, error) {
	lines := strings.Split(strings.ReplaceAll(patch, "\r\n", "\n"), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) < 3 || lines[0] != beginMarker || lines[len(lines)-1] != endMarker {
		return nil, fmt.Errorf("%w: patch must start with %q and end with %q", ErrInvalidPatch, beginMarker, endMarker)
	}

	var hunks []hunk
	for line := 1; line < len(lines)-1; {
		if lines[line] == "" {
			line++
			continue
		}
		if !strings.HasPrefix(lines[line], "@@") {
			return nil, fmt.Errorf("%w: line %d must start a hunk with @@", ErrInvalidPatch, line+1)
		}

		current := hunk{}
		header := strings.TrimSpace(strings.TrimPrefix(lines[line], "@@"))
		if header != "" {
			current.context = header
			current.hasContext = true
		}
		line++
		changed := false
		for line < len(lines)-1 && !strings.HasPrefix(lines[line], "@@") {
			entry := lines[line]
			switch {
			case entry == "":
				current.oldLines = append(current.oldLines, "")
				current.newLines = append(current.newLines, "")
			case strings.HasPrefix(entry, " "):
				current.oldLines = append(current.oldLines, entry[1:])
				current.newLines = append(current.newLines, entry[1:])
			case strings.HasPrefix(entry, "-"):
				current.oldLines = append(current.oldLines, entry[1:])
				changed = true
			case strings.HasPrefix(entry, "+"):
				current.newLines = append(current.newLines, entry[1:])
				changed = true
			default:
				return nil, fmt.Errorf("%w: line %d must begin with space, +, or -", ErrInvalidPatch, line+1)
			}
			line++
		}
		if !changed {
			return nil, fmt.Errorf("%w: hunk %d makes no change", ErrInvalidPatch, len(hunks)+1)
		}
		if len(current.oldLines) == 0 {
			return nil, fmt.Errorf("%w: hunk %d has no text to match", ErrInvalidPatch, len(hunks)+1)
		}
		hunks = append(hunks, current)
	}
	if len(hunks) == 0 {
		return nil, fmt.Errorf("%w: patch contains no hunks", ErrInvalidPatch)
	}
	return hunks, nil
}

func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func uniqueMatch(lines, pattern []string, start int) (int, error) {
	match := -1
	for index := start; index+len(pattern) <= len(lines); index++ {
		if equal(lines[index:index+len(pattern)], pattern) {
			if match >= 0 {
				return -1, ErrAmbiguous
			}
			match = index
		}
	}
	if match < 0 {
		return -1, ErrNoMatch
	}
	return match, nil
}

func equal(left, right []string) bool {
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func replace(lines []string, start, count int, replacement []string) []string {
	updated := make([]string, 0, len(lines)-count+len(replacement))
	updated = append(updated, lines[:start]...)
	updated = append(updated, replacement...)
	updated = append(updated, lines[start+count:]...)
	return updated
}
