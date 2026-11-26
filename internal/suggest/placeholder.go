package suggest

import (
	"regexp"
	"strings"
)

var placeholderRe = regexp.MustCompile(`<[a-zA-Z][\w-]*>`)

// placeholderSpans returns the [start, end) offsets of every placeholder in
// command. A match right after another '<' is shell syntax (a heredoc like
// <<EOF>), not a placeholder.
func placeholderSpans(command string) [][]int {
	var spans [][]int
	for _, m := range placeholderRe.FindAllStringIndex(command, -1) {
		if m[0] > 0 && command[m[0]-1] == '<' {
			continue
		}
		spans = append(spans, m)
	}
	return spans
}

// Placeholders returns the unique placeholder names in command, such as
// "file" for <file>, in the order they first appear.
func Placeholders(command string) []string {
	var names []string
	seen := map[string]bool{}
	for _, m := range placeholderSpans(command) {
		name := command[m[0]+1 : m[1]-1]
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	return names
}

// Fill replaces every placeholder with its value from values, quoting the
// value for the shell where needed. Placeholders without a value are left
// as they are.
func Fill(command string, values map[string]string) string {
	var b strings.Builder
	last := 0
	for _, m := range placeholderSpans(command) {
		v, ok := values[command[m[0]+1:m[1]-1]]
		if !ok {
			continue
		}
		b.WriteString(command[last:m[0]])
		b.WriteString(quoteIn(command, m[0], m[1], v))
		last = m[1]
	}
	b.WriteString(command[last:])
	return b.String()
}

// quoteIn quotes v for the spot [start, end) in command. When the
// placeholder already sits between quotes ('<x>' or "<x>"), only the
// characters that are special inside those quotes are escaped.
func quoteIn(command string, start, end int, v string) string {
	if start > 0 && end < len(command) {
		switch q := command[start-1]; {
		case q == '\'' && command[end] == '\'':
			return strings.ReplaceAll(v, "'", `'\''`)
		case q == '"' && command[end] == '"':
			r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "$", `\$`, "`", "\\`")
			return r.Replace(v)
		}
	}
	return Quote(v)
}

// Quote returns v as a single shell word. Values made only of safe
// characters are left alone, so "app.log" stays readable; anything else is
// wrapped in single quotes.
func Quote(v string) string {
	if v == "" {
		return "''"
	}
	safe := true
	for i, r := range v {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case strings.ContainsRune("_-./:=,+@%", r):
		case r == '~' && i == 0: // keep ~ expanding to the home directory
		default:
			safe = false
		}
	}
	if safe {
		return v
	}
	return "'" + strings.ReplaceAll(v, "'", `'\''`) + "'"
}
