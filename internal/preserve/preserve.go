// Package preserve implements custom code preservation for commandspec regeneration.
// Users can annotate generated files with // commandspec:custom:start / :end marker
// comments; Extract reads those blocks back out and Merge re-inserts them into
// freshly generated source.
package preserve

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	markerStart = "// commandspec:custom:start"
	markerEnd   = "// commandspec:custom:end"
)

// funcDeclRE matches the start of a Go function declaration line and captures
// the function name. It handles all common forms:
//
//	func Name(...)
//	func (r *Recv) Name(...)
var funcDeclRE = regexp.MustCompile(`^\s*func\s+(?:\([^)]*\)\s+)?(\w+)\s*\(`)

// CustomBlock represents a block of user-written code extracted from a
// commandspec:custom:start / :end marker pair.
type CustomBlock struct {
	// Label is the optional marker label (e.g. "my-hook").
	Label string
	// Content is the raw text between the start and end markers.
	Content string
	// Context is the name of the nearest enclosing function, or empty for
	// file-level blocks.
	Context string
}

// Extract scans source for commandspec:custom marker pairs and returns all
// custom blocks. It returns an error if a block is unclosed, nested, or if an
// end marker appears without a corresponding start marker.
func Extract(source string) ([]CustomBlock, error) {
	lines := strings.Split(source, "\n")
	var blocks []CustomBlock

	inBlock := false
	var blockLabel string
	var blockContext string
	var contentLines []string
	var startLineIdx int

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if isStartMarker(trimmed) {
			if inBlock {
				return nil, fmt.Errorf("nested custom block at line %d", i+1)
			}
			inBlock = true
			startLineIdx = i
			blockLabel = extractLabel(trimmed)
			blockContext = findEnclosingFunc(lines, i)
			contentLines = nil
			_ = startLineIdx
			continue
		}

		if isEndMarker(trimmed) {
			if !inBlock {
				return nil, fmt.Errorf("end marker without start at line %d", i+1)
			}
			blocks = append(blocks, CustomBlock{
				Label:   blockLabel,
				Content: strings.Join(contentLines, "\n"),
				Context: blockContext,
			})
			inBlock = false
			blockLabel = ""
			blockContext = ""
			contentLines = nil
			continue
		}

		if inBlock {
			contentLines = append(contentLines, strings.TrimRight(line, "\r"))
		}
	}

	if inBlock {
		return nil, fmt.Errorf("unclosed custom block starting at line %d", startLineIdx+1)
	}

	return blocks, nil
}

// Merge re-inserts preserved custom blocks into newSource. Matching is done by:
//  1. Label match — if the block has a label and newSource contains a marker
//     with the same label, insert the content there.
//  2. Context match — if the block has no label (or no label match), find a
//     marker in newSource inside a function with the same name.
//  3. Orphan — if no match is found, append the block as a warning comment at
//     the end of newSource.
//
// Merge returns newSource unchanged when blocks is nil or empty.
func Merge(newSource string, blocks []CustomBlock) (string, error) {
	if len(blocks) == 0 {
		return newSource, nil
	}

	// We work line-by-line on newSource, replacing the content between each
	// matched marker pair.
	lines := strings.Split(newSource, "\n")

	// Build a map from label → queue of blocks for ordered lookup.
	// Multiple blocks may share the same label; each label match in newSource
	// pops the next block from the front of the queue so all are placed in order.
	byLabel := make(map[string][]*CustomBlock)
	var unlabeled []*CustomBlock
	for i := range blocks {
		b := &blocks[i]
		if b.Label != "" {
			byLabel[b.Label] = append(byLabel[b.Label], b)
		} else {
			unlabeled = append(unlabeled, b)
		}
	}

	// Track which blocks have been placed so we can detect orphans.
	placed := make(map[*CustomBlock]bool)

	// Output accumulator.
	var out []string
	i := 0
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if isStartMarker(trimmed) {
			label := extractLabel(trimmed)
			context := findEnclosingFunc(lines, i)

			// Find a matching block.
			var match *CustomBlock
			if label != "" {
				if queue := byLabel[label]; len(queue) > 0 {
					match = queue[0]
					byLabel[label] = queue[1:]
				}
			} else {
				// Unlabeled: match by context.
				for _, b := range unlabeled {
					if !placed[b] && b.Context == context {
						match = b
						break
					}
				}
			}

			// Emit the start marker.
			out = append(out, line)
			i++

			// Skip over the existing content in newSource up to the end marker.
			for i < len(lines) {
				if isEndMarker(strings.TrimSpace(lines[i])) {
					break
				}
				if match == nil {
					// No match — keep existing content.
					out = append(out, lines[i])
				}
				i++
			}

			// Emit the replacement content if we have a match.
			if match != nil {
				placed[match] = true
				if match.Content != "" {
					// Content is stored without trailing newline on last line;
					// split and emit each content line.
					out = append(out, strings.Split(match.Content, "\n")...)
				}
			}

			// Emit the end marker (current line[i]).
			if i < len(lines) {
				out = append(out, lines[i])
				i++
			}
			continue
		}

		out = append(out, line)
		i++
	}

	// Append orphaned blocks as warning comments.
	var orphans []*CustomBlock
	for i := range blocks {
		if !placed[&blocks[i]] {
			orphans = append(orphans, &blocks[i])
		}
	}

	if len(orphans) > 0 {
		out = append(out, "")
		out = append(out, "// WARNING: The following custom blocks were orphaned during regeneration.")
		out = append(out, "// Their original location no longer exists. Review and relocate manually.")
		for _, b := range orphans {
			out = append(out, fmt.Sprintf("// commandspec:custom:start %s (orphaned from %q)", b.Label, b.Context))
			if b.Content != "" {
				for _, contentLine := range strings.Split(b.Content, "\n") {
					out = append(out, "// "+contentLine)
				}
			}
			out = append(out, "// commandspec:custom:end")
		}
	}

	return strings.Join(out, "\n"), nil
}

// isStartMarker reports whether a trimmed line is a custom block start marker.
func isStartMarker(trimmed string) bool {
	return strings.HasPrefix(trimmed, markerStart)
}

// isEndMarker reports whether a trimmed line is a custom block end marker.
func isEndMarker(trimmed string) bool {
	return strings.TrimSpace(trimmed) == markerEnd
}

// extractLabel returns the label portion of a start marker line, trimmed of
// leading/trailing whitespace. Returns empty string for unlabeled markers.
func extractLabel(trimmed string) string {
	after := strings.TrimPrefix(trimmed, markerStart)
	return strings.TrimSpace(after)
}

// findEnclosingFunc scans upward from lineIdx in lines to find the nearest
// enclosing function declaration and returns its name. Returns empty string if
// no enclosing function is found (file-level block).
func findEnclosingFunc(lines []string, lineIdx int) string {
	for i := lineIdx - 1; i >= 0; i-- {
		if m := funcDeclRE.FindStringSubmatch(lines[i]); m != nil {
			return m[1]
		}
	}
	return ""
}
