package metrics

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCalculateSinglePassMetricsAndComplexity(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/fast\n\ngo 1.27\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte(`package main

// Runner executes tasks.
type Runner interface {
	Run()
	Stop()
}

type App struct{}

const Version = "1.0"
var internalFlag = true

func (a *App) Run() {}

func main() {
	if internalFlag {
		println(Version)
	}
}
`), 0o644))

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "helper.go"), []byte(`package sub

type Helper interface{}

func Add(a, b int) int {
	return a + b
}
`), 0o644))

	require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "helper_test.go"), []byte(`package sub

import "testing"

func TestAdd(t *testing.T) {
	if Add(1, 2) != 3 {
		t.Fatal("bad")
	}
}
`), 0o644))

	res, err := Calculate(dir, "example.com/fast", Options{
		NeedBase:       true,
		NeedComplexity: true,
		TopN:           5,
	})
	require.NoError(t, err)
	require.NotNil(t, res.Base)
	require.NotNil(t, res.Complexity)

	require.Greater(t, res.Base.LOC.Total, 15)
	require.Equal(t, 2, res.Base.Types.Packages)
	require.Equal(t, 2, res.Base.Types.SourceFiles) // excludes helper_test.go
	require.Equal(t, 1, res.Base.Types.Structs)
	require.Equal(t, 1, res.Base.Types.PublicStructs)
	require.Equal(t, 1, res.Base.Types.Interfaces) // Helper has 0 methods
	require.Equal(t, 2, res.Base.Types.Functions)
	require.Equal(t, 1, res.Base.Types.PublicFunctions)
	require.Equal(t, 1, res.Base.Types.Methods)
	require.Equal(t, 1, res.Base.Types.PublicMethods)
	require.Equal(t, 1, res.Base.Types.Constants)
	require.Equal(t, 1, res.Base.Types.Variables)
	require.Equal(t, 0, res.Base.Types.PublicVariables)

	require.NotNil(t, res.Base.Tests)
	require.Equal(t, 1, res.Base.Tests.Files)
	require.Equal(t, 1, res.Base.Tests.Functions)
	require.Greater(t, res.Base.Tests.LOC, 5)
	require.Greater(t, res.Base.Tests.Ratio, 0.0)
	require.Len(t, res.Base.Tests.LargestFiles, 1)
	require.Equal(t, "helper_test.go", res.Base.Tests.LargestFiles[0].File)
	require.Equal(t, 1, res.Base.Tests.TestedPackages)
	require.Equal(t, 2, res.Base.Tests.TotalPackages)
	require.Equal(t, 50.0, res.Base.Tests.TestedPackagesRatio)
	require.Equal(t, []string{"example.com/fast"}, res.Base.Tests.UntestedPackages)

	require.Equal(t, 1.0, res.Base.Interfaces.AverageMethods)
	require.Len(t, res.Base.Interfaces.InterfaceMaxMethods, 1)
	require.Equal(t, "Runner", res.Base.Interfaces.InterfaceMaxMethods[0].Name)

	require.Equal(t, 1, res.Base.Comments.TotalLines)
	require.Equal(t, 1, res.Base.Comments.FilesWithComments)

	require.NotEmpty(t, res.Complexity.Cyclomatic.ComplexityDetails)
	require.Equal(t, "main", res.Complexity.Cyclomatic.ComplexityDetails[0].Function)

	require.NotNil(t, res.Base.Functions)
	require.Equal(t, 3, res.Base.Functions.Total)
	require.Equal(t, 3.0, res.Base.Functions.AverageLOC)
	require.Equal(t, 5, res.Base.Functions.MaxLOC)
	require.Equal(t, 0.67, res.Base.Functions.AverageParams)
	require.Equal(t, 2, res.Base.Functions.MaxParams)
	require.Equal(t, "main", res.Base.Functions.LongFunctions[0].Name)
	require.Len(t, res.Base.Functions.HighParamFuncs, 1)
	require.Equal(t, "Add", res.Base.Functions.HighParamFuncs[0].Name)

	require.NotNil(t, res.Base.DocCoverage)
	require.Equal(t, 5, res.Base.DocCoverage.ExportedTotal)
	require.Equal(t, 1, res.Base.DocCoverage.Documented)
	require.Equal(t, 20.0, res.Base.DocCoverage.Ratio)
	require.Len(t, res.Base.DocCoverage.Undocumented, 4)
}
