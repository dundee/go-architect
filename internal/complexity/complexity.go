package complexity

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
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
