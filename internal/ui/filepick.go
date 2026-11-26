package ui

import (
	"os"
	"strings"
)

// maxMatches is how many directory entries the picker shows at once.
const maxMatches = 8

// isFilePlaceholder reports whether a placeholder name like "logfile" or
// "src_path" asks for something on disk, so the form offers a file picker.
func isFilePlaceholder(name string) bool {
	n := strings.ToLower(name)
	for _, w := range []string{"file", "path", "dir", "log", "src", "dest"} {
		if strings.Contains(n, w) {
			return true
		}
	}
	return false
}

// wantsDir reports whether a placeholder only makes sense as a directory,
// like <dir> or <target-dir>, but not <dir-or-file>.
func wantsDir(name string) bool {
	n := strings.ToLower(name)
	return strings.Contains(n, "dir") && !strings.Contains(n, "file")
}

// listEntries returns the names in dir, folders with a trailing "/". It
// returns nil when dir can't be read.
func listEntries(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		names = append(names, name)
	}
	return names
}

// filterEntries returns the entries matching query, case-insensitively:
// names starting with it first, then names containing it, each in the
// original order. Hidden entries only show once the query starts with ".".
// With dirsOnly, files are left out.
func filterEntries(entries []string, query string, dirsOnly bool) []string {
	q := strings.ToLower(query)
	var prefix, contains []string
	for _, e := range entries {
		if strings.HasPrefix(e, ".") && !strings.HasPrefix(q, ".") {
			continue
		}
		if dirsOnly && !strings.HasSuffix(e, "/") {
			continue
		}
		l := strings.ToLower(e)
		switch {
		case strings.HasPrefix(l, q):
			prefix = append(prefix, e)
		case strings.Contains(l, q):
			contains = append(contains, e)
		}
	}
	return append(prefix, contains...)
}
