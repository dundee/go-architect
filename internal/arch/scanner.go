package arch

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io/fs"
	"math"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
)

type scannedFile struct {
	pkgPath         string
	fileName        string
	imports         []string
	importWeights   map[string]int64
	abstractions    int
	implementations int
}

type scannedPackage struct {
	pkgPath         string
	imports         []string
	importWeights   map[string]int64
	abstractions    int
	implementations int
}

type fileTarget struct {
	absPath  string
	pkgPath  string
	fileName string
}

func scanPackages(projectPath, mainPackage string, needSelectorWeights, includeTests bool) ([]scannedPackage, error) {
	absRoot, err := filepath.Abs(projectPath)
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
		if !includeTests && strings.HasSuffix(name, "_test.go") {
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
		pkgLabel := mainPackage
		if pkgLabel == "" {
			pkgLabel = filepath.Base(dir)
		} else if relDir != "." {
			pkgLabel = path.Join(mainPackage, filepath.ToSlash(relDir))
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
		return []scannedPackage{}, nil
	}

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

	results := make([][]scannedFile, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(workerID int) {
			defer wg.Done()
			var local []scannedFile
			for target := range jobs {
				if sf, ok := parseScannedFile(target, needSelectorWeights); ok {
					local = append(local, sf)
				}
			}
			results[workerID] = local
		}(w)
	}
	wg.Wait()

	pkgMap := make(map[string]*scannedPackage)
	importSets := make(map[string]map[string]struct{})
	for _, batch := range results {
		for _, sf := range batch {
			sp, ok := pkgMap[sf.pkgPath]
			if !ok {
				sp = &scannedPackage{
					pkgPath:       sf.pkgPath,
					importWeights: make(map[string]int64),
				}
				pkgMap[sf.pkgPath] = sp
				importSets[sf.pkgPath] = make(map[string]struct{})
			}
			sp.abstractions += sf.abstractions
			sp.implementations += sf.implementations
			for _, imp := range sf.imports {
				importSets[sf.pkgPath][imp] = struct{}{}
			}
			for imp, w := range sf.importWeights {
				sp.importWeights[imp] += w
			}
		}
	}

	out := make([]scannedPackage, 0, len(pkgMap))
	for pkgPath, sp := range pkgMap {
		imps := make([]string, 0, len(importSets[pkgPath]))
		for imp := range importSets[pkgPath] {
			imps = append(imps, imp)
		}
		sort.Strings(imps)
		sp.imports = imps
		out = append(out, *sp)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].pkgPath < out[j].pkgPath
	})
	return out, nil
}

func parseScannedFile(target fileTarget, needSelectorWeights bool) (scannedFile, bool) {
	data, err := os.ReadFile(target.absPath)
	if err != nil {
		return scannedFile{}, false
	}

	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, target.fileName, data, 0)
	if err != nil {
		return scannedFile{}, false
	}

	sf := scannedFile{
		pkgPath:  target.pkgPath,
		fileName: target.fileName,
	}

	aliasToPkg := make(map[string]string, len(astFile.Imports))
	var weights map[string]int64
	if needSelectorWeights {
		weights = make(map[string]int64, len(astFile.Imports))
	}

	for _, imp := range astFile.Imports {
		if imp.Path == nil {
			continue
		}
		impPath := strings.Trim(imp.Path.Value, `"`)
		sf.imports = append(sf.imports, impPath)
		if needSelectorWeights {
			weights[impPath]++
			if imp.Name != nil {
				if imp.Name.Name != "." && imp.Name.Name != "_" {
					aliasToPkg[imp.Name.Name] = impPath
				}
			} else {
				aliasToPkg[defaultPkgPrefix(impPath)] = impPath
			}
		}
	}

	for _, decl := range astFile.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			sf.implementations++
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				switch tx := ts.Type.(type) {
				case *ast.StructType:
					sf.abstractions++
				case *ast.InterfaceType:
					if tx.Methods != nil && len(tx.Methods.List) > 0 {
						sf.abstractions++
					}
				}
			}
		}
	}

	if needSelectorWeights && len(aliasToPkg) > 0 {
		ast.Inspect(astFile, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if id, ok := sel.X.(*ast.Ident); ok {
				if pkg, matched := aliasToPkg[id.Name]; matched {
					weights[pkg]++
				}
			}
			return true
		})
		sf.importWeights = weights
	}

	return sf, true
}

func defaultPkgPrefix(dep string) string {
	if idx := strings.LastIndexByte(dep, '/'); idx >= 0 {
		return dep[idx+1:]
	}
	return dep
}

func isInternalPackage(pkg, mainPackage string) bool {
	if mainPackage == "" {
		return false
	}
	return pkg == mainPackage || strings.HasPrefix(pkg, mainPackage+"/")
}

func isOrganizationPackage(pkg string, orgPatterns []string) bool {
	for _, op := range orgPatterns {
		if op != "" && strings.HasPrefix(pkg, op) {
			return true
		}
	}
	return false
}

func isStandardPackage(pkg string) bool {
	if strings.HasPrefix(pkg, "golang.org/x") {
		return true
	}
	return !strings.ContainsAny(pkg, ".")
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
