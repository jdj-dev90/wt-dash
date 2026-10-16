package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// listOutput is the subset of `wt list --format json` (schema 2) we render.
// See https://worktrunk.dev/schema/list-v2.json.
type listOutput struct {
	Schema int `json:"schema"`
	Repo   struct {
		DefaultBranch string `json:"default_branch"`
	} `json:"repo"`
	Items []item `json:"items"`
}

type item struct {
	Branch *string `json:"branch"`
	Head   *struct {
		ShortSHA    string  `json:"short_sha"`
		Subject     *string `json:"subject"`
		CommittedAt *string `json:"committed_at"`
	} `json:"head"`
	Worktree *struct {
		Path    string `json:"path"`
		Main    bool   `json:"main"`
		Changes struct {
			Staged     bool `json:"staged"`
			Modified   bool `json:"modified"`
			Untracked  bool `json:"untracked"`
			Conflicted bool `json:"conflicted"`
			Diff       *struct {
				Added   int `json:"added"`
				Deleted int `json:"deleted"`
			} `json:"diff"`
		} `json:"changes"`
	} `json:"worktree"`
	DefaultBranch *struct {
		Ahead  int `json:"ahead"`
		Behind int `json:"behind"`
	} `json:"default_branch"`
	Upstream *struct {
		Ahead  int `json:"ahead"`
		Behind int `json:"behind"`
	} `json:"upstream"`
	PR *struct {
		Number *int    `json:"number"`
		URL    *string `json:"url"`
		Review string  `json:"review"`
	} `json:"pr"`
	Checks *struct {
		Status *string `json:"status"`
	} `json:"checks"`
	Marker  string `json:"marker"`
	Display struct {
		Symbols string `json:"symbols"`
	} `json:"display"`
}

func (it item) branchName() string {
	if it.Branch == nil {
		return "(detached)"
	}
	return *it.Branch
}

func (it item) path() string {
	if it.Worktree == nil {
		return ""
	}
	return it.Worktree.Path
}

func (it item) isMain() bool { return it.Worktree != nil && it.Worktree.Main }

func (it item) subject() string {
	if it.Head == nil || it.Head.Subject == nil {
		return ""
	}
	return *it.Head.Subject
}

func (it item) committedAt() (time.Time, bool) {
	if it.Head == nil || it.Head.CommittedAt == nil {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, *it.Head.CommittedAt)
	return t, err == nil
}

func (it item) prURL() string {
	if it.PR == nil || it.PR.URL == nil {
		return ""
	}
	return *it.PR.URL
}

type repoData struct {
	Path          string
	Name          string
	DefaultBranch string
	Items         []item
	Err           error
}

// wtList runs `wt list --full` for one repo. CI/MR data is cached by wt, so
// polling this every few seconds stays cheap.
func wtList(ctx context.Context, path string) repoData {
	rd := repoData{Path: path, Name: filepath.Base(path)}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "wt", "-C", path, "list", "--full", "--format", "json").Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			err = fmt.Errorf("%s", strings.TrimSpace(lastLine(string(ee.Stderr))))
		}
		rd.Err = err
		return rd
	}
	parsed, err := parseList(out)
	if err != nil {
		rd.Err = err
		return rd
	}
	rd.DefaultBranch = parsed.Repo.DefaultBranch
	for _, it := range parsed.Items {
		if it.Worktree != nil { // worktree rows only; skip branch-only rows
			rd.Items = append(rd.Items, it)
		}
	}
	return rd
}

func parseList(b []byte) (listOutput, error) {
	var lo listOutput
	if err := json.Unmarshal(b, &lo); err != nil {
		return lo, fmt.Errorf("parse wt list json: %w", err)
	}
	if lo.Schema != 2 {
		return lo, fmt.Errorf("unsupported wt list schema %d (want 2)", lo.Schema)
	}
	return lo, nil
}

func lastLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		return s[i+1:]
	}
	return s
}
