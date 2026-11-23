package complexity

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCalculateInProcess(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/demo\n\ngo 1.24\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte(`package main

func simple() int {
	return 1
}

func branching(x int) int {
	if x > 0 {
		for i := 0; i < x; i++ {
			if i%2 == 0 {
				return i
			}
		}
	}
	return 0
}
`), 0o644))

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "helper.go"), []byte(`package sub

func helper(a, b bool) int {
	if a && b {
		return 1
	}
	return 0
}
`), 0o644))

	// Test files and vendor should be ignored
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main_test.go"), []byte(`package main
func TestHuge() {
	if true { if true { if true { if true {} } } }
}
`), 0o644))

	report, err := Calculate(dir, "example.com/demo", 2)
	require.NoError(t, err)
	require.NotNil(t, report)

	require.Greater(t, report.Cyclomatic.AverageComplexity, 1.0)
	require.Len(t, report.Cyclomatic.ComplexityDetails, 2)
	require.Equal(t, "branching", report.Cyclomatic.ComplexityDetails[0].Function)
	require.Equal(t, "example.com/demo", report.Cyclomatic.ComplexityDetails[0].Package)

	require.Greater(t, report.Cognitive.AverageComplexity, 0.0)
	require.Len(t, report.Cognitive.ComplexityDetails, 2)
	require.Equal(t, "branching", report.Cognitive.ComplexityDetails[0].Function)
}

func TestCalculateWorkerPoolDeterministic(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/many\n\ngo 1.24\n"), 0o644))

	for i := 0; i < 24; i++ {
		sub := filepath.Join(dir, "pkg", string(rune('a'+i)))
		require.NoError(t, os.MkdirAll(sub, 0o755))
		src := "package p\nfunc fn(x int) int {\n\tif x > 0 {\n\t\treturn 1\n\t}\n\treturn 0\n}\n"
		require.NoError(t, os.WriteFile(filepath.Join(sub, "f.go"), []byte(src), 0o644))
	}

	first, err := Calculate(dir, "example.com/many", 5)
	require.NoError(t, err)
	for run := 0; run < 5; run++ {
		next, err := Calculate(dir, "example.com/many", 5)
		require.NoError(t, err)
		require.Equal(t, first, next)
	}
}
