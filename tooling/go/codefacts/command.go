package codefacts

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// Run captures portable facts from root, builds the envelope, and writes pure JSON to stdout.
func Run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("code-facts", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "repository root")
	repositoryID := flags.String("repository-id", "", "workspace repository identifier")
	worktreeID := flags.String("worktree-id", "", "workspace worktree identifier")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "code-facts accepts only flags")
		return 2
	}
	snapshot, err := Capture(*root, DefaultPolicy)
	if err != nil {
		fmt.Fprintln(stderr, "code-facts:", err)
		return 1
	}
	if *repositoryID == "" {
		*repositoryID = gitValue(*root, "rev-parse", "--git-common-dir")
		if *repositoryID == "" {
			*repositoryID = "local"
		}
	}
	if *worktreeID == "" {
		*worktreeID = gitValue(*root, "rev-parse", "--git-dir")
		if *worktreeID == "" {
			*worktreeID = "default"
		}
	}
	envelope, err := Build(snapshot, Options{RepositoryID: *repositoryID, WorktreeID: *worktreeID})
	if err != nil {
		fmt.Fprintln(stderr, "code-facts:", err)
		return 1
	}
	encoder := json.NewEncoder(stdout)
	if err := encoder.Encode(envelope); err != nil {
		fmt.Fprintln(stderr, "code-facts:", err)
		return 1
	}
	return 0
}

// gitValue returns one trimmed git configuration value or an empty string.
func gitValue(root string, arguments ...string) string {
	command := exec.Command("git", append([]string{"-C", root}, arguments...)...)
	output, err := command.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}
