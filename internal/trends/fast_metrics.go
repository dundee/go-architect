package trends

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io"
	"math"
	"path"
	"runtime"
	"strings"
	"sync"

	"github.com/dundee/go-architect/internal/complexity"
	"github.com/fzipp/gocyclo"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/uudashr/gocognit"
	"golang.org/x/mod/modfile"
)

type treeFile struct {
	relPath string
	dir     string
	name    string
	content []byte
}

type parsedFileMetrics struct {
	dir              string
	pkgLabel         string
	isTest           bool
	loc              int
	testFunctions    int
	commentLines     int
	structs          int
	interfaces       int
	functions        int
	methods          int
	fnLOCSum         int
	fnParamSum       int
	interfaceMethods []int
	imports          []string
	cycloDetails     []complexity.Detail
	cognitDetails    []complexity.Detail
	cycloSum         int
	cognitSum        int
}

func measureCommitFromTree(
	repo *git.Repository,
	relPrefix string,
	c sampledCommit,
	includeComplexity bool,
) (*CommitSnapshot, *complexity.Report, error) {
	commit, err := repo.CommitObject(plumbing.NewHash(c.SHA))
	if err != nil {
		return nil, nil, fmt.Errorf("load commit %s: %w", c.Short, err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, nil, fmt.Errorf("load tree %s: %w", c.Short, err)
	}

	goModPath := "go.mod"
	if relPrefix != "" {
		goModPath = relPrefix + "/go.mod"
	}
	goModFile, err := tree.File(goModPath)
	if err != nil {
		return nil, nil, fmt.Errorf("no go.mod at commit")
	}
	goModContent, err := goModFile.Contents()
	if err != nil {
		return nil, nil, fmt.Errorf("read go.mod: %w", err)
	}
	modPath := modfile.ModulePath([]byte(goModContent))
	if modPath == "" {
		return nil, nil, fmt.Errorf("invalid go.mod without module path")
	}

	filesIter := tree.Files()
	defer filesIter.Close()

	var candidates []treeFile
	byPath := make(map[string][]byte)
	prefixSlash := ""
	if relPrefix != "" {
		prefixSlash = relPrefix + "/"
	}

	err = filesIter.ForEach(func(f *object.File) error {
		relToMod := f.Name
		if prefixSlash != "" {
			if !strings.HasPrefix(f.Name, prefixSlash) {
				return nil
			}
			relToMod = strings.TrimPrefix(f.Name, prefixSlash)
		}
		if !strings.HasSuffix(relToMod, ".go") || shouldSkipRelPath(relToMod) {
			return nil
		}
		raw, readErr := f.Contents()
		if readErr != nil {
			return nil
		}
		data := []byte(raw)
		dir := path.Dir(relToMod)
		name := path.Base(relToMod)
		byPath[relToMod] = data
		candidates = append(candidates, treeFile{
			relPath: relToMod,
			dir:     dir,
			name:    name,
			content: data,
		})
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("walk tree files: %w", err)
	}

	// Filter using go/build constraints completely in-memory.
	buildCtx := newTreeBuildContext(byPath)

	matched := make([]treeFile, 0, len(candidates))
	for _, tf := range candidates {
		ok, matchErr := buildCtx.MatchFile(tf.dir, tf.name)
		if matchErr == nil && ok {
			matched = append(matched, tf)
		}
	}

	metrics, compReport := analyzeTreeFiles(matched, modPath, includeComplexity)

	return &CommitSnapshot{
		CommitSHA: c.SHA,
		ShortSHA:  c.Short,
		Date:      c.Date,
		Message:   c.Message,
		Tag:       c.Tag,
		Metrics:   metrics,
		Cached:    false,
	}, compReport, nil
}

func newTreeBuildContext(byPath map[string][]byte) build.Context {
	buildCtx := build.Default
	buildCtx.JoinPath = path.Join
	buildCtx.OpenFile = func(p string) (io.ReadCloser, error) {
		normalized := strings.ReplaceAll(p, "\\", "/")
		clean := strings.TrimPrefix(path.Clean(normalized), "./")
		data, ok := byPath[clean]
		if !ok {
			return nil, fmt.Errorf("file not found: %s", p)
		}
		return io.NopCloser(bytes.NewReader(data)), nil
	}
	return buildCtx
}

func measureComplexityFromTree(
	repo *git.Repository,
	relPrefix string,
	sha string,
) (*complexity.Report, error) {
	snap, compReport, err := measureCommitFromTree(repo, relPrefix, sampledCommit{SHA: sha, Short: sha[:min(7, len(sha))]}, true)
	if err != nil {
		return nil, err
	}
	_ = snap
	return compReport, nil
}

func shouldSkipRelPath(relPath string) bool {
	parts := strings.Split(relPath, "/")
	// Check directory segments (all except final filename).
	for _, seg := range parts[:len(parts)-1] {
		if strings.HasPrefix(seg, ".") || seg == "vendor" || seg == "node_modules" || seg == "testdata" {
			return true
		}
	}
	return false
}

func analyzeTreeFiles(files []treeFile, modPath string, includeComplexity bool) (SnapshotMetrics, *complexity.Report) {
	if len(files) == 0 {
		var compReport *complexity.Report
		if includeComplexity {
			compReport = &complexity.Report{
				Cyclomatic: complexity.Output{ComplexityDetails: []complexity.Detail{}},
				Cognitive:  complexity.Output{ComplexityDetails: []complexity.Detail{}},
			}
		}
		return SnapshotMetrics{}, compReport
	}

	workers := runtime.GOMAXPROCS(0)
	if workers < 1 {
		workers = 1
	}
	if workers > len(files) {
		workers = len(files)
	}

	jobs := make(chan treeFile, len(files))
	for _, f := range files {
		jobs <- f
	}
	close(jobs)

	results := make([][]parsedFileMetrics, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(workerID int) {
			defer wg.Done()
			var local []parsedFileMetrics
			for tf := range jobs {
				if parsed, ok := parseSingleTreeFile(tf, modPath, includeComplexity); ok {
					local = append(local, parsed)
				}
			}
			results[workerID] = local
		}(w)
	}
	wg.Wait()

	type pkgStats struct {
		abstractions    int
		implementations int
		efferent        map[string]struct{}
		afferent        map[string]struct{}
	}

	pkgMap := make(map[string]*pkgStats)
	ensurePkg := func(pkgPath string) *pkgStats {
		st, ok := pkgMap[pkgPath]
		if !ok {
			st = &pkgStats{
				efferent: make(map[string]struct{}),
				afferent: make(map[string]struct{}),
			}
			pkgMap[pkgPath] = st
		}
		return st
	}

	var (
		totalLOC, sourceFiles                  int
		testLOC, testFiles, testFunctions      int
		structs, interfaces, functions, methods int
		fnLOCSum, fnParamSum                   int
		commentsTotal                          int
		ifaceMethodSum, ifaceCount             int
		cycloDetails, cognitDetails            []complexity.Detail
		cycloSum, cognitSum                    int
	)

	// Collect all package names first (excluding *_test.go-only packages) so afferent/efferent only tracks production packages.
	var allParsed []parsedFileMetrics
	for _, batch := range results {
		for _, pf := range batch {
			if pf.isTest {
				testFiles++
				testLOC += pf.loc
				testFunctions += pf.testFunctions
				continue
			}
			ensurePkg(pf.pkgLabel)
			allParsed = append(allParsed, pf)
		}
	}

	for _, pf := range allParsed {
		sourceFiles++
		totalLOC += pf.loc
		commentsTotal += pf.commentLines
		structs += pf.structs
		interfaces += pf.interfaces
		functions += pf.functions
		methods += pf.methods
		fnLOCSum += pf.fnLOCSum
		fnParamSum += pf.fnParamSum
		for _, mCount := range pf.interfaceMethods {
			ifaceCount++
			ifaceMethodSum += mCount
		}

		st := pkgMap[pf.pkgLabel]
		st.abstractions += pf.structs + pf.interfaces
		st.implementations += pf.functions + pf.methods

		for _, imp := range pf.imports {
			if strings.HasPrefix(imp, modPath) {
				st.efferent[imp] = struct{}{}
				if target, exists := pkgMap[imp]; exists {
					target.afferent[pf.pkgLabel] = struct{}{}
				}
			}
		}

		if includeComplexity {
			cycloSum += pf.cycloSum
			cognitSum += pf.cognitSum
			cycloDetails = append(cycloDetails, pf.cycloDetails...)
			cognitDetails = append(cognitDetails, pf.cognitDetails...)
		}
	}

	var distanceSum float64
	painCount := 0
	uselessCount := 0
	for _, st := range pkgMap {
		ce := len(st.efferent)
		ca := len(st.afferent)
		var instab, abstr float64
		if ca+ce > 0 {
			instab = round2(float64(ce) / float64(ca+ce))
		}
		if st.implementations+st.abstractions > 0 {
			abstr = round2(float64(st.abstractions) / float64(st.implementations+st.abstractions))
		}
		dist := round2(math.Abs(instab + abstr - 1))
		distanceSum += dist
		if dist >= 0.4 {
			if abstr+instab < 1 {
				painCount++
			} else {
				uselessCount++
			}
		}
	}

	var avgDistance float64
	if len(pkgMap) > 0 {
		avgDistance = distanceSum / float64(len(pkgMap))
	}

	var commentsRatio float64
	if totalLOC > 0 {
		commentsRatio = round2(100 * float64(commentsTotal) / float64(totalLOC))
	}

	var ifaceAvgMethods float64
	if ifaceCount > 0 {
		ifaceAvgMethods = float64(ifaceMethodSum) / float64(ifaceCount)
	}

	var avgFuncLOC, avgFuncParams float64
	if totalFuncs := functions + methods; totalFuncs > 0 {
		avgFuncLOC = round2(float64(fnLOCSum) / float64(totalFuncs))
		avgFuncParams = round2(float64(fnParamSum) / float64(totalFuncs))
	}

	var compReport *complexity.Report
	if includeComplexity {
		compReport = &complexity.Report{
			Cyclomatic: complexity.Summarize(cycloDetails, cycloSum, 5),
			Cognitive:  complexity.Summarize(cognitDetails, cognitSum, 5),
		}
	}

	return SnapshotMetrics{
		LOC:                    totalLOC,
		Packages:               len(pkgMap),
		SourceFiles:            sourceFiles,
		Structs:                structs,
		Interfaces:             interfaces,
		Functions:              functions,
		Methods:                methods,
		CommentsTotalLines:     commentsTotal,
		CommentsRatio:          commentsRatio,
		InterfaceAvgMethods:    ifaceAvgMethods,
		AvgDistance:            avgDistance,
		ZoneOfPainCount:        painCount,
		ZoneOfUselessnessCount: uselessCount,
		TestLOC:                testLOC,
		TestFiles:              testFiles,
		TestFunctions:          testFunctions,
		AvgFunctionLOC:         avgFuncLOC,
		AvgFunctionParams:      avgFuncParams,
	}, compReport
}

func parseSingleTreeFile(tf treeFile, modPath string, includeComplexity bool) (parsedFileMetrics, bool) {
	isTest := strings.HasSuffix(tf.name, "_test.go")
	parseMode := parser.ParseComments
	if isTest {
		parseMode = 0
	}

	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, tf.name, tf.content, parseMode)
	if err != nil {
		return parsedFileMetrics{}, false
	}

	pkgLabel := modPath
	if tf.dir != "." && tf.dir != "" {
		pkgLabel = path.Join(modPath, tf.dir)
	}

	var out parsedFileMetrics
	out.dir = tf.dir
	out.pkgLabel = pkgLabel
	out.isTest = isTest
	out.loc = countLinesBytes(tf.content)

	if isTest {
		for _, decl := range astFile.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && isTestFuncName(fn.Name.Name) {
				out.testFunctions++
			}
		}
		return out, true
	}

	for _, cg := range astFile.Comments {
		out.commentLines += len(cg.List)
	}

	for _, imp := range astFile.Imports {
		if imp.Path != nil {
			out.imports = append(out.imports, strings.Trim(imp.Path.Value, `"`))
		}
	}

	for _, decl := range astFile.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Recv != nil {
				out.methods++
			} else {
				out.functions++
			}
			fnLOC := fset.Position(d.End()).Line - fset.Position(d.Pos()).Line + 1
			if fnLOC < 1 {
				fnLOC = 1
			}
			out.fnLOCSum += fnLOC
			out.fnParamSum += countFuncParams(d.Type)
		case *ast.GenDecl:
			if len(d.Specs) == 0 {
				continue
			}
			for _, spec := range d.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				switch tx := ts.Type.(type) {
				case *ast.StructType:
					out.structs++
				case *ast.InterfaceType:
					mCount := 0
					if tx.Methods != nil {
						mCount = len(tx.Methods.List)
					}
					out.interfaceMethods = append(out.interfaceMethods, mCount)
					if mCount > 0 {
						out.interfaces++
					}
				}
			}
		}
	}

	if includeComplexity && !strings.HasSuffix(tf.name, "_test.go") {
		cStats := gocyclo.AnalyzeASTFile(astFile, fset, nil)
		for _, st := range cStats {
			out.cycloSum += st.Complexity
			out.cycloDetails = append(out.cycloDetails, complexity.Detail{
				Package:    pkgLabel,
				File:       fmt.Sprintf("%s:%d", tf.relPath, st.Pos.Line),
				Function:   st.FuncName,
				Complexity: st.Complexity,
			})
		}

		gStats := gocognit.ComplexityStats(astFile, fset, nil)
		for _, st := range gStats {
			out.cognitSum += st.Complexity
			out.cognitDetails = append(out.cognitDetails, complexity.Detail{
				Package:    pkgLabel,
				File:       fmt.Sprintf("%s:%d", tf.relPath, st.Pos.Line),
				Function:   st.FuncName,
				Complexity: st.Complexity,
			})
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

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func isTestFuncName(name string) bool {
	return strings.HasPrefix(name, "Test") ||
		strings.HasPrefix(name, "Benchmark") ||
		strings.HasPrefix(name, "Fuzz") ||
		strings.HasPrefix(name, "Example")
}

func countFuncParams(ft *ast.FuncType) int {
	if ft == nil || ft.Params == nil {
		return 0
	}
	total := 0
	for _, f := range ft.Params.List {
		if len(f.Names) == 0 {
			total++
		} else {
			total += len(f.Names)
		}
	}
	return total
}
