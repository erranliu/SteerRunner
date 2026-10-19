package resultacquisition

import (
	"net/url"
	"regexp"
	"strings"
)

var urlPattern = regexp.MustCompile(`https?://[^\s<>"()\[\]{}` + "`" + `]+`)

// PRURLs only locates clues. Provider identity/state verification remains with
// the original publication owner and must follow even a success declaration.
func PRURLs(text string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, s := range urlPattern.FindAllString(text, -1) {
		// Natural-language finals may wrap the URL in Markdown emphasis.
		// Remove presentation suffixes before deduplication and provider lookup;
		// this still supplies only a clue, never a publication verdict.
		s = strings.TrimRight(s, ".,;:!?) ]}。；，*_")
		u, err := url.Parse(s)
		if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			continue
		}
		if !strings.Contains(u.Path, "/pull/") && !strings.Contains(u.Path, "/pulls/") {
			continue
		}
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func PublicationClues(final string, candidates []string) []string {
	out := PRURLs(final)
	seen := map[string]bool{}
	for _, s := range out {
		seen[s] = true
	}
	for _, candidate := range candidates {
		for _, s := range PRURLs(candidate) {
			if s == candidate && !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	}
	return out
}
