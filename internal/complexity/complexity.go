package complexity

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"path/filepath"
	"sort"
	"strings"

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

func Calculate(projectPath, mainPackage string, topN int) (*Report, error) {
	if topN <= 0 {
		topN = 5
	}

	absRoot, err := filepath.Abs(projectPath)
	if err != nil {
		return nil, fmt.Errorf("resolve project path: %w", err)
	}

	fset := token.NewFileSet()
	var cycloDetails []Detail
	var cognitDetails []Detail
	var cycloSum, cognitSum int

	err = filepath.WalkDir(absRoot, func(filePath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
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

		astFile, parseErr := parser.ParseFile(fset, filePath, nil, 0)
		if parseErr != nil {
			// Skip unparseable/generated scratch files gracefully.
			return nil
		}

		pkgLabel := resolvePackageName(absRoot, filePath, mainPackage, astFile.Name.Name)
		relFile, relErr := filepath.Rel(absRoot, filePath)
		if relErr != nil {
			relFile = name
		}
		relFileSlash := filepath.ToSlash(relFile)

		cStats := gocyclo.AnalyzeASTFile(astFile, fset, nil)
		for _, st := range cStats {
			cycloSum += st.Complexity
			cycloDetails = append(cycloDetails, Detail{
				Package:    pkgLabel,
				File:       fmt.Sprintf("%s:%d", relFileSlash, st.Pos.Line),
				Function:   st.FuncName,
				Complexity: st.Complexity,
			})
		}

		gStats := gocognit.ComplexityStats(astFile, fset, nil)
		for _, st := range gStats {
			cognitSum += st.Complexity
			cognitDetails = append(cognitDetails, Detail{
				Package:    pkgLabel,
				File:       fmt.Sprintf("%s:%d", relFileSlash, st.Pos.Line),
				Function:   st.FuncName,
				Complexity: st.Complexity,
			})
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk project files: %w", err)
	}

	return &Report{
		Cyclomatic: summarize(cycloDetails, cycloSum, topN),
		Cognitive:  summarize(cognitDetails, cognitSum, topN),
	}, nil
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

func summarize(all []Detail, sum, topN int) Output {
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
