package main

import (
	"os"
	"os/exec"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// pause keeps a failed command's output on screen until a key is pressed.
const pauseOnFail = `status=$?; if [ $status -ne 0 ]; then printf '\n\033[33mexit %s, press enter\033[0m'; read -r _; fi; exit $status`

type execDoneMsg struct {
	what string
	err  error
}

// targetClient picks the tmux client to open sessions on: the most recently
// active client that isn't showing the dashboard's own session. Empty means
// "the current client" (e.g. single monitor, or not in tmux).
func targetClient() string {
	if os.Getenv("TMUX") == "" {
		return ""
	}
	own, err := exec.Command("tmux", "display-message", "-p", "#{session_name}").Output()
	if err != nil {
		return ""
	}
	ownSession := strings.TrimSpace(string(own))

	out, err := exec.Command("tmux", "list-clients", "-F", "#{client_name}\t#{client_activity}\t#{session_name}").Output()
	if err != nil {
		return ""
	}
	best, bestAct := "", int64(-1)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.Split(line, "\t")
		if len(f) != 3 || f[2] == ownSession {
			continue
		}
		act, _ := strconv.ParseInt(f[1], 10, 64)
		if act > bestAct {
			best, bestAct = f[0], act
		}
	}
	return best
}

func withClientEnv(cmd *exec.Cmd) *exec.Cmd {
	cmd.Env = os.Environ()
	if c := targetClient(); c != "" {
		cmd.Env = append(cmd.Env, "TMUX_SESSIONIZER_CLIENT="+c)
	}
	return cmd
}

// openWorktree jumps to (or creates) the worktree's tmux-sessionizer session.
func openWorktree(path string) tea.Cmd {
	return func() tea.Msg {
		cmd := withClientEnv(exec.Command("tmux-sessionizer", path))
		out, err := cmd.CombinedOutput()
		if err != nil && len(out) > 0 {
			err = execErr(lastLine(string(out)))
		}
		return execDoneMsg{what: "open " + path, err: err}
	}
}

// newWorktree hands the terminal to `wt switch --create` so hook output and
// approval prompts are visible, then opens the new worktree's session.
func newWorktree(repo, branch string) tea.Cmd {
	script := `wt -C "$1" switch --create "$2" --no-cd -x tmux-sessionizer -- '{{ worktree_path }}'; ` + pauseOnFail
	cmd := withClientEnv(exec.Command("sh", "-c", script, "sh", repo, branch))
	return tea.ExecProcess(cmd, func(err error) tea.Msg { return execDoneMsg{what: "new " + branch, err: err} })
}

// removeWorktree runs `wt remove`, which refuses unmerged branches by itself.
func removeWorktree(repo, branch string) tea.Cmd {
	script := `wt -C "$1" remove "$2"; ` + pauseOnFail
	cmd := exec.Command("sh", "-c", script, "sh", repo, branch)
	return tea.ExecProcess(cmd, func(err error) tea.Msg { return execDoneMsg{what: "remove " + branch, err: err} })
}

// showDiff pages everything on the branch since it left the default branch,
// committed and uncommitted, through git's configured pager.
func showDiff(path, defaultBranch string) tea.Cmd {
	script := `base=$(git -C "$1" merge-base HEAD "origin/$2" 2>/dev/null || git -C "$1" merge-base HEAD "$2" 2>/dev/null || echo HEAD)
git -C "$1" -c color.ui=always diff "$base"; ` + pauseOnFail
	cmd := exec.Command("sh", "-c", script, "sh", path, defaultBranch)
	return tea.ExecProcess(cmd, func(err error) tea.Msg { return execDoneMsg{what: "diff", err: err} })
}

func openURL(url string) tea.Cmd {
	return func() tea.Msg {
		err := exec.Command("xdg-open", url).Start()
		return execDoneMsg{what: "open MR", err: err}
	}
}

func notify(title, body string) {
	_ = exec.Command("notify-send", "-a", "wt-dash", title, body).Start()
}

type execErr string

func (e execErr) Error() string { return string(e) }
