package agentlaunch

import (
	"net/url"
	"regexp"
	"strings"
)

var gitHubRepositoryPart = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

func gitHubRepository(repository string) bool {
	u, err := url.Parse(repository)
	if err != nil || u.Scheme != "https" || !strings.EqualFold(u.Host, "github.com") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	return len(parts) == 2 && gitHubRepositoryPart.MatchString(parts[0]) && gitHubRepositoryPart.MatchString(parts[1])
}
