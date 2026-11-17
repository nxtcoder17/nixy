package yaml_ast

import "strings"

// Format puts a single blank line between top-level mapping entries.
// Nested nodes, block scalar content, and comments that head a top-level key
// are left untouched.
func Format(b []byte) []byte {
	lines := strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")

	out := make([]string, 0, len(lines)+len(lines)/4)
	blockIndent := -1
	headCommentSeparated := false

	for i, line := range lines {
		indent := indentOf(line)

		if blockIndent >= 0 && (line == "" || indent > blockIndent) {
			out = append(out, line)
			continue
		}
		blockIndent = -1

		if startsTopLevelBlock(lines, i) {
			isComment := strings.HasPrefix(line, "#")
			if !headCommentSeparated && needsBlankLine(out) {
				out = append(out, "")
			}
			headCommentSeparated = isComment
		}

		if isBlockScalarStart(line) {
			blockIndent = indent
		}

		out = append(out, line)
	}

	return []byte(strings.Join(out, "\n") + "\n")
}

// startsTopLevelBlock reports whether lines[i] opens a top-level mapping
// entry, counting the comment block that documents it as part of that entry.
func startsTopLevelBlock(lines []string, i int) bool {
	if indentOf(lines[i]) != 0 {
		return false
	}

	if strings.HasPrefix(lines[i], "#") {
		for _, line := range lines[i+1:] {
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			return isKeyLine(line)
		}
		return false
	}

	return isKeyLine(lines[i])
}

func isKeyLine(line string) bool {
	if line == "" || line == "---" || line == "..." {
		return false
	}
	return indentOf(line) == 0 && line[0] != '-' && line[0] != '#'
}

func isBlockScalarStart(line string) bool {
	_, value, ok := strings.Cut(line, ":")
	if !ok {
		return false
	}

	switch strings.TrimSpace(value) {
	case "|", ">", "|-", ">-", "|+", ">+":
		return true
	default:
		return false
	}
}

func indentOf(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}

func needsBlankLine(out []string) bool {
	last := len(out) - 1
	return last >= 0 && out[last] != "" && out[last] != "---"
}
