package normalize

import (
	"strings"
	"unicode"
)

// Name collapses whitespace and lowercases for stable matching (ASCII-focused MVP).
func Name(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !prevSpace && b.Len() > 0 {
				b.WriteRune(' ')
				prevSpace = true
			}
			continue
		}
		prevSpace = false
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}
