// Package gitcmd builds git subprocess invocations hardened against
// repository-local configuration that could execute arbitrary commands.
package gitcmd

import (
	"context"
	"os/exec"
)

// safeConfig disables repository-controlled settings that make git spawn
// helper programs during read-only commands (e.g. `git status`). Analyzed
// Repositories are not necessarily trusted, so their .git/config must not be
// able to run code on the user's machine.
var safeConfig = []string{
	"-c", "core.fsmonitor=false",
	"-c", "core.hooksPath=" + nullDevice,
	"-c", "diff.external=",
	"-c", "core.pager=cat",
}

// Command returns an exec.Cmd running `git -C dir <args...>` with hardened config.
func Command(dir string, args ...string) *exec.Cmd {
	return exec.Command("git", buildArgs(dir, args)...)
}

// CommandContext is like Command but bound to ctx.
func CommandContext(ctx context.Context, dir string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "git", buildArgs(dir, args)...)
}

func buildArgs(dir string, args []string) []string {
	full := make([]string, 0, len(safeConfig)+2+len(args))
	full = append(full, safeConfig...)
	full = append(full, "-C", dir)
	return append(full, args...)
}
