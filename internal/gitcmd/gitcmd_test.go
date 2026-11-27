package gitcmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommandPrependsSafeConfig(t *testing.T) {
	cmd := Command("/repo", "status", "--porcelain")
	args := cmd.Args[1:]
	assert.Equal(t, safeConfig, args[:len(safeConfig)])
	assert.Equal(t, []string{"-C", "/repo", "status", "--porcelain"}, args[len(safeConfig):])
}

func TestStatusIgnoresMaliciousFSMonitor(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fsmonitor not portable to Windows")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "pwned")
	run := func(args ...string) {
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
		require.NoError(t, err, string(out))
	}
	run("init", "-q")
	run("config", "core.fsmonitor", "touch "+marker+"; false")

	out, err := CommandContext(t.Context(), dir, "status", "--porcelain").CombinedOutput()
	require.NoError(t, err, string(out))
	_, statErr := os.Stat(marker)
	assert.True(t, os.IsNotExist(statErr), "fsmonitor hook must not execute")
}
