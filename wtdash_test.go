package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

const sampleList = `{
  "schema": 2,
  "repo": {"default_branch": "main"},
  "collected": {"ci": true, "summary": false},
  "items": [
    {"branch": "main",
     "head": {"short_sha": "b187c8c6", "subject": "chore: bump", "committed_at": "2026-10-16T01:58:41Z"},
     "worktree": {"path": "/r/app", "main": true, "changes": {"untracked": true, "diff": {"added": 5756, "deleted": 0}}},
     "upstream": {"remote": "origin", "branch": "main", "ahead": 0, "behind": 0},
     "checks": {"status": "running", "source": "branch", "stale": false},
     "display": {"symbols": "?^|"}},
    {"branch": "PROJ-123",
     "head": {"short_sha": "05a4a45", "subject": "feat: login", "committed_at": "2026-10-16T00:00:00Z"},
     "worktree": {"path": "/r/app.PROJ-123", "main": false, "changes": {"modified": true, "diff": {"added": 42, "deleted": 3}}},
     "default_branch": {"ahead": 2, "behind": 1},
     "pr": {"number": 118, "url": "https://git.example.com/app/-/merge_requests/118", "review": "approved"},
     "checks": {"status": "passed", "source": "pr", "stale": false},
     "marker": "💬",
     "display": {"symbols": "!↕💬"}},
    {"branch": "old-branch", "head": {"short_sha": "abc"}, "display": {"symbols": ""}}
  ]
}`

func TestParseList(t *testing.T) {
	lo, err := parseList([]byte(sampleList))
	if err != nil {
		t.Fatal(err)
	}
	if lo.Repo.DefaultBranch != "main" || len(lo.Items) != 3 {
		t.Fatalf("unexpected envelope: %+v", lo.Repo)
	}
	it := lo.Items[1]
	if it.branchName() != "PROJ-123" || it.path() != "/r/app.PROJ-123" || it.isMain() {
		t.Errorf("identity fields wrong: %q %q %v", it.branchName(), it.path(), it.isMain())
	}
	if it.prURL() == "" || *it.PR.Number != 118 || it.Marker != "💬" {
		t.Errorf("pr/marker wrong: %+v %q", it.PR, it.Marker)
	}
	if got := stripMarker(it.Display.Symbols, it.Marker); got != "!↕" {
		t.Errorf("stripMarker = %q", got)
	}
	if !strings.Contains(fmtCI(it), "!118") {
		t.Errorf("fmtCI missing MR number: %q", fmtCI(it))
	}
}

func TestParseListRejectsOtherSchema(t *testing.T) {
	if _, err := parseList([]byte(`[{"branch":"x"}]`)); err == nil {
		t.Fatal("expected error for schema-1 bare array")
	}
}

func TestReadRepoList(t *testing.T) {
	dir := t.TempDir()
	home, _ := os.UserHomeDir()
	list := filepath.Join(dir, ".wt-dash")
	content := "# client repos\nshop-api\n  shop-contracts   # trailing comment\n\n/abs/repo\n~/code/x\n"
	if err := os.WriteFile(list, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := readRepoList(list)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(dir, "shop-api"),
		filepath.Join(dir, "shop-contracts"),
		"/abs/repo",
		filepath.Join(home, "code/x"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
}

func TestFindRepoListWalksUp(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "client", "repo", "sub")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	list := filepath.Join(root, "client", ".wt-dash")
	if err := os.WriteFile(list, []byte("repo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := findRepoList("", deep)
	if err != nil || got != list {
		t.Errorf("walk up: got %q, %v; want %q", got, err, list)
	}
	got, _ = findRepoList(filepath.Join(root, "client"), "/")
	if got != list {
		t.Errorf("dir arg: got %q want %q", got, list)
	}
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "cfg"))
	got, _ = findRepoList("", root)
	if got != filepath.Join(root, "cfg", "wt-dash", "repos") {
		t.Errorf("fallback: got %q", got)
	}
}

func TestApplyDataKeepsCursorAndNotifiesOnWaiting(t *testing.T) {
	lo, _ := parseList([]byte(sampleList))
	working := lo.Items[1]
	working.Marker = markerWorking
	rd := func(it item) []repoData {
		return []repoData{{Path: "/r/app", Name: "app", Items: []item{lo.Items[0], it}}}
	}

	m := newModel([]string{"/r/app"}, time.Second, false)
	m.applyData(rd(working))
	m.cursor = 1

	// marker flips to waiting; cursor must stay on PROJ-123 even if rows reorder
	m.applyData([]repoData{{Path: "/r/app", Name: "app", Items: []item{lo.Items[1], lo.Items[0]}}})
	if _, it, _ := m.selected(); it.branchName() != "PROJ-123" {
		t.Errorf("cursor moved to %q", it.branchName())
	}
	if m.markers["/r/app|PROJ-123"] != markerWaiting {
		t.Errorf("marker not tracked: %v", m.markers)
	}
}

func TestHumanAge(t *testing.T) {
	cases := map[time.Duration]string{
		30 * time.Second:    "now",
		5 * time.Minute:     "5m",
		3 * time.Hour:       "3h",
		2 * 24 * time.Hour:  "2d",
		30 * 24 * time.Hour: "4w",
	}
	for d, want := range cases {
		if got := humanAge(d); got != want {
			t.Errorf("humanAge(%v) = %q, want %q", d, got, want)
		}
	}
}
