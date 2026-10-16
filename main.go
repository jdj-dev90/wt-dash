// wt-dash is a live dashboard for Worktrunk (wt) worktrees across several
// repos: agent status, diffs, ahead/behind and MR/CI at a glance, with keys to
// open a worktree's tmux session, create, remove, diff and open the MR.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

var version = "dev"

func main() {
	var (
		list     = flag.Bool("list", false, "print the resolved repo paths and exit")
		interval = flag.Duration("interval", 5*time.Second, "refresh interval")
		noNotify = flag.Bool("no-notify", false, "disable desktop notifications when an agent starts waiting")
		showVer  = flag.Bool("version", false, "print version and exit")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `usage: wt-dash [flags] [file|dir]

Repo list (one path per line, '#' comments, relative to the file), first match wins:
  1. [file|dir] argument (dir -> dir/.wt-dash)
  2. nearest .wt-dash walking up from the current directory
  3. $XDG_CONFIG_HOME/wt-dash/repos

flags:
`)
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVer {
		fmt.Println(version)
		return
	}

	cwd, _ := os.Getwd()
	listFile, err := findRepoList(flag.Arg(0), cwd)
	if err != nil {
		fail(err)
	}
	repos, err := readRepoList(listFile)
	if err != nil {
		fail(err)
	}

	if *list {
		for _, r := range repos {
			fmt.Println(r)
		}
		return
	}

	if _, err := exec.LookPath("wt"); err != nil {
		fail(fmt.Errorf("wt (worktrunk) not found on PATH"))
	}

	p := tea.NewProgram(newModel(repos, *interval, !*noNotify), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "wt-dash:", err)
	os.Exit(1)
}
