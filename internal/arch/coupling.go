package arch

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/go-architect/go-architect-lib/coupling"
	"github.com/go-architect/go-architect-lib/project"
)

type couplingFileResult struct {
	pkgPath string
	file    *coupling.FileCoupling
}

func CalculateCoupling(proj *project.ProjectInfo, dep string) (*coupling.DependencyCoupling, error) {
	absRoot, err := filepath.Abs(proj.Path)
	if err != nil {
		return nil, fmt.Errorf("resolve project path: %w", err)
	}

	var targets []fileTarget
	buildCtx := build.Default
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
		if !strings.HasSuffix(name, ".go") {
			return nil
		}

		dir := filepath.Dir(filePath)
		ok, matchErr := buildCtx.MatchFile(dir, name)
		if matchErr != nil || !ok {
			return nil
		}

		relDir, relErr := filepath.Rel(absRoot, dir)
		if relErr != nil {
			relDir = "."
		}
		pkgLabel := proj.Package
		if pkgLabel == "" {
			pkgLabel = filepath.Base(dir)
		} else if relDir != "." {
			pkgLabel = path.Join(proj.Package, filepath.ToSlash(relDir))
		}

		targets = append(targets, fileTarget{
			absPath:  filePath,
			pkgPath:  pkgLabel,
			fileName: name,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk project files: %w", err)
	}

	if len(targets) == 0 {
		return &coupling.DependencyCoupling{
			Dependency:     dep,
			CouplingLevel:  0,
			PackageDetails: []*coupling.PackageCoupling{},
		}, nil
	}

	depNeedle := []byte(dep)
	workers := runtime.GOMAXPROCS(0)
	if workers < 1 {
		workers = 1
	}
	if workers > len(targets) {
		workers = len(targets)
	}

	jobs := make(chan fileTarget, len(targets))
	for _, t := range targets {
		jobs <- t
	}
	close(jobs)

	results := make([][]couplingFileResult, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(workerID int) {
			defer wg.Done()
			var local []couplingFileResult
			for target := range jobs {
				if fc := inspectFileCoupling(target, dep, depNeedle); fc != nil {
					local = append(local, couplingFileResult{
						pkgPath: target.pkgPath,
						file:    fc,
					})
				}
			}
			results[workerID] = local
		}(w)
	}
	wg.Wait()

	byPkg := make(map[string]*coupling.PackageCoupling)
	for _, batch := range results {
		for _, item := range batch {
			pc, ok := byPkg[item.pkgPath]
			if !ok {
				pc = &coupling.PackageCoupling{
					Package: item.pkgPath,
				}
				byPkg[item.pkgPath] = pc
			}
			pc.FileDetails = append(pc.FileDetails, item.file)
			pc.CouplingLevel += item.file.CouplingLevel
		}
	}

	pkgDetails := make([]*coupling.PackageCoupling, 0, len(byPkg))
	totalLevel := 0
	for _, pc := range byPkg {
		sort.Slice(pc.FileDetails, func(i, j int) bool {
			if pc.FileDetails[i].CouplingLevel != pc.FileDetails[j].CouplingLevel {
				return pc.FileDetails[i].CouplingLevel > pc.FileDetails[j].CouplingLevel
			}
			return pc.FileDetails[i].File < pc.FileDetails[j].File
		})
		totalLevel += pc.CouplingLevel
		pkgDetails = append(pkgDetails, pc)
	}

	sort.Slice(pkgDetails, func(i, j int) bool {
		if pkgDetails[i].CouplingLevel != pkgDetails[j].CouplingLevel {
			return pkgDetails[i].CouplingLevel > pkgDetails[j].CouplingLevel
		}
		return pkgDetails[i].Package < pkgDetails[j].Package
	})

	return &coupling.DependencyCoupling{
		Dependency:     dep,
		CouplingLevel:  totalLevel,
		PackageDetails: pkgDetails,
	}, nil
}

func inspectFileCoupling(target fileTarget, dep string, depNeedle []byte) *coupling.FileCoupling {
	data, err := os.ReadFile(target.absPath)
	if err != nil {
		return nil
	}
	// Fast substring pre-check avoids AST parsing on files that cannot import dep.
	if !bytes.Contains(data, depNeedle) {
		return nil
	}

	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, target.fileName, data, 0)
	if err != nil {
		return nil
	}

	var prefix string
	imported := false
	var details []coupling.Detail

	for _, imp := range astFile.Imports {
		if imp.Path == nil {
			continue
		}
		if strings.Trim(imp.Path.Value, `"`) == dep {
			imported = true
			if imp.Name != nil && imp.Name.Name != "." && imp.Name.Name != "_" {
				prefix = imp.Name.Name
			} else if imp.Name == nil {
				prefix = defaultPkgPrefix(dep)
			}
			details = append(details, coupling.Detail{
				Line:     fset.Position(imp.Pos()).Line,
				Comments: "Imports package",
			})
		}
	}

	if !imported {
		return nil
	}

	if prefix != "" {
		ast.Inspect(astFile, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == prefix {
				var buf bytes.Buffer
				_ = printer.Fprint(&buf, fset, sel)
				details = append(details, coupling.Detail{
					Line:     fset.Position(sel.Pos()).Line,
					Comments: fmt.Sprintf("Call to '%s'", buf.String()),
				})
			}
			return true
		})
	}

	lineSeen := make(map[int]struct{}, len(details))
	lines := make([]int, 0, len(details))
	for _, d := range details {
		if _, ok := lineSeen[d.Line]; !ok {
			lineSeen[d.Line] = struct{}{}
			lines = append(lines, d.Line)
		}
	}

	return &coupling.FileCoupling{
		Package:       target.pkgPath,
		File:          target.fileName,
		FileContent:   string(data),
		Lines:         lines,
		CouplingLevel: len(details),
		Details:       details,
	}
}
