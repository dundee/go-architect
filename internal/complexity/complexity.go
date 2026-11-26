package complexity

import (
	"bytes"
	"fmt"
	"go/build"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/fzipp/gocyclo"
	"github.com/uudashr/gocognit"
)

type Output struct {
	AverageComplexity float64  `json:"average_complexity"`
	ComplexityDetails []Detail `json:"ComplexityDetails"`
}

type Detail struct {
	Package    string `json:"Package"`
	File       string `json:"File"`
	Function   string `json:"Function"`
	Complexity int    `json:"Complexity"`
}

type Report struct {
	Cyclomatic Output
	Cognitive  Output
}

type FileStats struct {
	Package         string
	RelFile         string
	LOC             int
	CyclomaticTotal int
	CognitiveTotal  int
	MaxCyclomatic   int
	MaxCognitive    int
	Functions       int
}

type workerResult struct {
	cycloDetails  []Detail
	cognitDetails []Detail
	cycloSum      int
	cognitSum     int
}

func Calculate(projectPath, mainPackage string, topN int) (*Report, error) {
	if topN <= 0 {
		topN = 5
	}

	absRoot, err := filepath.Abs(projectPath)
	if err != nil {
		return nil, fmt.Errorf("resolve project path: %w", err)
	}

	workers := runtime.GOMAXPROCS(0)
	if workers < 1 {
		workers = 1
	}

	jobs := make(chan string, workers*4)
	results := make([]workerResult, workers)

	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(workerID int) {
			defer wg.Done()
			var local workerResult
			for filePath := range jobs {
				processFile(absRoot, filePath, mainPackage, &local)
			}
			results[workerID] = local
		}(w)
	}

	walkErr := filepath.WalkDir(absRoot, func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		name := d.Name()
		if d.IsDir() {
			if filePath != absRoot && (strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" || name == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}

		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}

		jobs <- filePath
		return nil
	})
	close(jobs)
	wg.Wait()

	if walkErr != nil {
		return nil, fmt.Errorf("walk project files: %w", walkErr)
	}

	var cycloDetails []Detail
	var cognitDetails []Detail
	var cycloSum, cognitSum int
	for _, r := range results {
		cycloSum += r.cycloSum
		cognitSum += r.cognitSum
		cycloDetails = append(cycloDetails, r.cycloDetails...)
		cognitDetails = append(cognitDetails, r.cognitDetails...)
	}

	return &Report{
		Cyclomatic: Summarize(cycloDetails, cycloSum, topN),
		Cognitive:  Summarize(cognitDetails, cognitSum, topN),
	}, nil
}

func processFile(absRoot, filePath, mainPackage string, out *workerResult) {
	// Per-file FileSet avoids shared mutex contention and keeps memory bounded.
	fset := token.NewFileSet()
	astFile, parseErr := parser.ParseFile(fset, filePath, nil, 0)
	if parseErr != nil {
		// Skip unparseable/generated scratch files gracefully.
		return
	}

	pkgLabel := resolvePackageName(absRoot, filePath, mainPackage, astFile.Name.Name)
	relFile, relErr := filepath.Rel(absRoot, filePath)
	if relErr != nil {
		relFile = filepath.Base(filePath)
	}
	relFileSlash := filepath.ToSlash(relFile)

	cStats := gocyclo.AnalyzeASTFile(astFile, fset, nil)
	for _, st := range cStats {
		out.cycloSum += st.Complexity
		out.cycloDetails = append(out.cycloDetails, Detail{
			Package:    pkgLabel,
			File:       fmt.Sprintf("%s:%d", relFileSlash, st.Pos.Line),
			Function:   st.FuncName,
			Complexity: st.Complexity,
		})
	}

	gStats := gocognit.ComplexityStats(astFile, fset, nil)
	for _, st := range gStats {
		out.cognitSum += st.Complexity
		out.cognitDetails = append(out.cognitDetails, Detail{
			Package:    pkgLabel,
			File:       fmt.Sprintf("%s:%d", relFileSlash, st.Pos.Line),
			Function:   st.FuncName,
			Complexity: st.Complexity,
		})
	}
}

func resolvePackageName(projectRoot, filePath, mainPackage, astPkg string) string {
	if mainPackage == "" {
		return astPkg
	}
	relDir, err := filepath.Rel(projectRoot, filepath.Dir(filePath))
	if err != nil || relDir == "." {
		return mainPackage
	}
	return path.Join(mainPackage, filepath.ToSlash(relDir))
}

func Summarize(all []Detail, sum, topN int) Output {
	if len(all) == 0 {
		return Output{
			AverageComplexity: 0,
			ComplexityDetails: []Detail{},
		}
	}

	sort.Slice(all, func(i, j int) bool {
		if all[i].Complexity != all[j].Complexity {
			return all[i].Complexity > all[j].Complexity
		}
		if all[i].Package != all[j].Package {
			return all[i].Package < all[j].Package
		}
		if all[i].File != all[j].File {
			return all[i].File < all[j].File
		}
		return all[i].Function < all[j].Function
	})

	limit := topN
	if len(all) < limit {
		limit = len(all)
	}

	return Output{
		AverageComplexity: float64(sum) / float64(len(all)),
		ComplexityDetails: all[:limit],
	}
}

func CalculatePerFile(projectPath, mainPackage string) (map[string]FileStats, error) {
	absRoot, err := filepath.Abs(projectPath)
	if err != nil {
		return nil, fmt.Errorf("resolve project path: %w", err)
	}

	buildCtx := build.Default
	var files []string
	walkErr := filepath.WalkDir(absRoot, func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		name := d.Name()
		if d.IsDir() {
			if filePath != absRoot && (strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" || name == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}

		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}

		ok, matchErr := buildCtx.MatchFile(filepath.Dir(filePath), name)
		if matchErr != nil || !ok {
			return nil
		}

		files = append(files, filePath)
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("walk project files: %w", walkErr)
	}

	if len(files) == 0 {
		return map[string]FileStats{}, nil
	}

	workers := runtime.GOMAXPROCS(0)
	if workers < 1 {
		workers = 1
	}
	if workers > len(files) {
		workers = len(files)
	}

	jobs := make(chan string, len(files))
	for _, f := range files {
		jobs <- f
	}
	close(jobs)

	results := make([][]FileStats, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(workerID int) {
			defer wg.Done()
			var local []FileStats
			for filePath := range jobs {
				if fsItem, ok := processFileStats(absRoot, filePath, mainPackage); ok {
					local = append(local, fsItem)
				}
			}
			results[workerID] = local
		}(w)
	}
	wg.Wait()

	out := make(map[string]FileStats, len(files))
	for _, batch := range results {
		for _, item := range batch {
			out[item.RelFile] = item
		}
	}
	return out, nil
}

func processFileStats(absRoot, filePath, mainPackage string) (FileStats, bool) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return FileStats{}, false
	}

	fset := token.NewFileSet()
	astFile, parseErr := parser.ParseFile(fset, filepath.Base(filePath), data, 0)
	if parseErr != nil {
		return FileStats{}, false
	}

	pkgLabel := resolvePackageName(absRoot, filePath, mainPackage, astFile.Name.Name)
	relFile, relErr := filepath.Rel(absRoot, filePath)
	if relErr != nil {
		relFile = filepath.Base(filePath)
	}
	relFileSlash := filepath.ToSlash(relFile)

	out := FileStats{
		Package: pkgLabel,
		RelFile: relFileSlash,
		LOC:     countLinesBytes(data),
	}

	cStats := gocyclo.AnalyzeASTFile(astFile, fset, nil)
	out.Functions = len(cStats)
	for _, st := range cStats {
		out.CyclomaticTotal += st.Complexity
		if st.Complexity > out.MaxCyclomatic {
			out.MaxCyclomatic = st.Complexity
		}
	}

	gStats := gocognit.ComplexityStats(astFile, fset, nil)
	if len(gStats) > out.Functions {
		out.Functions = len(gStats)
	}
	for _, st := range gStats {
		out.CognitiveTotal += st.Complexity
		if st.Complexity > out.MaxCognitive {
			out.MaxCognitive = st.Complexity
		}
	}

	return out, true
}

func countLinesBytes(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	count := bytes.Count(data, []byte{'\n'})
	if count == 0 {
		return 0
	}
	return count + 1
}
