package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// repoListName is the per-directory file listing repos, e.g. one per client dir.
const repoListName = ".wt-dash"

// findRepoList resolves which repo list to use, first match wins:
//  1. arg: a list file, or a directory holding a .wt-dash
//  2. the nearest .wt-dash walking up from start
//  3. $XDG_CONFIG_HOME/wt-dash/repos (master fallback)
func findRepoList(arg, start string) (string, error) {
	if arg != "" {
		if fi, err := os.Stat(arg); err == nil && fi.IsDir() {
			arg = filepath.Join(arg, repoListName)
		}
		return arg, nil
	}

	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, repoListName)
		if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	cfg := os.Getenv("XDG_CONFIG_HOME")
	if cfg == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		cfg = filepath.Join(home, ".config")
	}
	return filepath.Join(cfg, "wt-dash", "repos"), nil
}

// readRepoList parses one repo path per line. '#' starts a comment, '~' expands
// to $HOME, and relative paths resolve from the list file's directory.
func readRepoList(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("no repo list at %s (one repo path per line)", path)
		}
		return nil, err
	}
	defer f.Close()

	base := filepath.Dir(path)
	home, _ := os.UserHomeDir()

	var repos []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line == "~" || strings.HasPrefix(line, "~/") {
			line = filepath.Join(home, strings.TrimPrefix(line, "~"))
		}
		if !filepath.IsAbs(line) {
			line = filepath.Join(base, line)
		}
		repos = append(repos, filepath.Clean(line))
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(repos) == 0 {
		return nil, fmt.Errorf("no repos listed in %s", path)
	}
	return repos, nil
}
