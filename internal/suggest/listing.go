package suggest

import (
	"fmt"
	"os"
	"strings"
)

// maxListing is how many entries of the current directory go into the prompt.
const maxListing = 50

// usefulHidden are dotfiles worth showing the model; other hidden entries
// (.git, .DS_Store, .idea…) are noise.
var usefulHidden = map[string]bool{
	".env": true, ".env.local": true, ".env.example": true,
	".gitignore": true, ".dockerignore": true, ".htaccess": true,
	".editorconfig": true,
}

// dirListing returns the entries of dir as a comma-separated list, folders
// with a trailing "/". It returns "" when dir can't be read or is empty.
func dirListing(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var names []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") && !usefulHidden[name] {
			continue
		}
		if e.IsDir() {
			name += "/"
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return ""
	}
	more := len(names) - maxListing
	if more > 0 {
		names = names[:maxListing]
	}
	s := strings.Join(names, ", ")
	if more > 0 {
		s += fmt.Sprintf(", … and %d more", more)
	}
	return s
}
