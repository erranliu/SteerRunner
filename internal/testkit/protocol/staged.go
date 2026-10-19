package testprotocol

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// StagedBusiness is the existing executable fixture's deterministic model
// substitute. Its artifacts are real Git facts; each native input does only
// the action for its bound step. It is never reachable from production code.
func StagedBusiness(cwd, prompt, provider, remote string) (string, error) {
	prURL := "https://controlled.invalid/owner/repo/pull/84"
	if os.Getenv("STEERRUNNER_FIXTURE_MULTI_PROVIDER") == "1" {
		sum := sha256.Sum256([]byte(cwd))
		number := binary.BigEndian.Uint32(sum[:4]) + 1
		prURL = fmt.Sprintf("https://controlled.invalid/owner/repo/pull/%d", number)
		provider = filepath.Join(filepath.Dir(provider), fmt.Sprintf("pr-%d.json", number))
	}
	git := func(args ...string) (string, error) {
		data, err := exec.Command("git", append([]string{"-C", cwd}, args...)...).CombinedOutput()
		if err != nil {
			return "", errors.New(string(data))
		}
		return strings.TrimSpace(string(data)), nil
	}
	switch {
	case strings.HasPrefix(strings.TrimSpace(prompt), "Implement the following Spec and Plan.") || strings.HasPrefix(prompt, "根据以下 Spec、Plan"):
		content := "staged fixture result\n"
		if err := os.WriteFile(filepath.Join(cwd, "stage-result.txt"), []byte(content), 0600); err != nil {
			return "", err
		}
		if _, err := git("add", "--", "stage-result.txt"); err != nil {
			return "", err
		}
		if _, err := git("-c", "user.name=Stage Fixture", "-c", "user.email=stage@example.invalid", "-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "staged candidate"); err != nil {
			return "", err
		}
		if os.Getenv("STEERRUNNER_SUCCESS_OUTPUT") == "1" {
			return "Implemented and verified.\n实施及必要验证已完成。", nil
		}
		return "Implemented and verified.", nil
	case strings.HasPrefix(strings.TrimSpace(prompt), "Format the preceding implementation result as JSON.") || strings.HasPrefix(prompt, "根据已有执行上下文整理指定实施结果"):
		return `{"outcome":"done","summary":"source implementation completed"}`, nil
	case strings.HasPrefix(strings.TrimSpace(prompt), "Format the preceding publication result as JSON.") || strings.HasPrefix(prompt, "根据已有上下文整理指定发布"):
		// Fixture model memory comes from the provider artifact of its preceding turn.
		data, err := os.ReadFile(provider)
		if err != nil {
			return "", err
		}
		var result struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			return "", err
		}
		url := result.URL
		if url == "" {
			return "", errors.New("publication report has no original PR URL")
		}

		body, err := json.Marshal(map[string]any{"outcome": "done", "outputs": map[string]string{"pr_url": url}, "summary": "original publication reported"})
		return string(body), err
	case strings.HasPrefix(strings.TrimSpace(prompt), "Push and create or reuse a PR using the current checked-out branch.") || strings.HasPrefix(prompt, "将当前分支 push"):
		head, err := git("rev-parse", "HEAD")
		if err != nil {
			return "", err
		}
		branch, err := git("symbolic-ref", "--short", "HEAD")
		if err != nil {
			return "", err
		}
		if _, err := git("push", remote, "HEAD:refs/heads/"+branch); err != nil {
			return "", err
		}
		body, _ := json.Marshal(map[string]any{"url": prURL, "repository": "owner/repo", "base_branch": "main", "head_repository": "owner/repo", "head_branch": branch, "head_revision": head, "state": "open", "merged": false})
		if err := os.WriteFile(provider, body, 0600); err != nil {
			return "", err
		}
		if os.Getenv("STEERRUNNER_SUCCESS_OUTPUT") == "1" {
			return "PR: " + prURL + "\n本次发布已完成。", nil
		}
		if os.Getenv("STEERRUNNER_STAGE_PUBLISH_UNKNOWN") == "1" {
			return `{"outcome":"blocked","summary":"publication uncertain","outputs":{"pr_url":"https://controlled.invalid/owner/repo/pull/84"},"blocker":{"category":"unknown_external","action":"publish","error":"response lost"}}`, nil
		}
		if os.Getenv("STEERRUNNER_STAGE_PUBLISH_PLAIN") == "1" {
			return "Publication completed; URL needs reporting from existing context.", nil
		}
		result, _ := json.Marshal(map[string]any{"outcome": "done", "outputs": map[string]string{"pr_url": prURL}, "summary": "candidate pushed and PR created"})
		return string(result), nil
	}
	return "", errors.New("fixture received an unknown stage")
}
