package metrics

import (
	"bytes"
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
	"unicode"

	"github.com/dundee/go-architect/internal/cache"
	"github.com/dundee/go-architect/internal/complexity"
	"github.com/fzipp/gocyclo"
	"github.com/go-architect/go-architect-lib/metrics/comments"
	"github.com/go-architect/go-architect-lib/metrics/interfaces"
	"github.com/go-architect/go-architect-lib/metrics/loc"
	"github.com/go-architect/go-architect-lib/metrics/types"
	"github.com/uudashr/gocognit"
)

type Options struct {
	NeedBase       bool
	NeedComplexity bool
	TopN           int
}

type Result struct {
	Base       *cache.MetricsBase
	Complexity *complexity.Report
}

type fileWork struct {
	absPath  string
	relFile  string
	pkgPath  string
	fileName string
	isTest   bool
}

type fileMetrics struct {
	work          fileWork
	loc           int
	commentLines  int
	testFunctions int
	counts        typesCounts
	ifaceInfos    []interfaces.InterfaceInfo
	cycloDetails  []complexity.Detail
	cognitDetails []complexity.Detail
	cycloSum      int
	cognitSum     int
}

type typesCounts struct {
	structs          int
	interfaces       int
	functions        int
	methods          int
	variables        int
	constants        int
	publicStructs    int
	publicInterfaces int
	publicFunctions  int
	publicMethods    int
	publicVariables  int
	publicConstants  int
}

func Calculate(projectPath, mainPackage string, opts Options) (*Result, error) {
	if opts.TopN <= 0 {
		opts.TopN = 5
	}
	if !opts.NeedBase && !opts.NeedComplexity {
		return &Result{}, nil
	}

	absRoot, err := filepath.Abs(projectPath)
	if err != nil {
		return nil, fmt.Errorf("resolve project path: %w", err)
	}

	var files []fileWork
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

		isTest := strings.HasSuffix(name, "_test.go")
		if !opts.NeedBase && isTest {
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

		relFile, err := filepath.Rel(absRoot, filePath)
		if err != nil {
			relFile = name
		}

		files = append(files, fileWork{
			absPath:  filePath,
			relFile:  filepath.ToSlash(relFile),
			pkgPath:  pkgLabel,
			fileName: name,
			isTest:   isTest,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk project files: %w", err)
	}

	workers := runtime.GOMAXPROCS(0)
	if workers < 1 {
		workers = 1
	}
	if len(files) > 0 && workers > len(files) {
		workers = len(files)
	}

	jobs := make(chan fileWork, len(files))
	for _, f := range files {
		jobs <- f
	}
	close(jobs)

	results := make([][]fileMetrics, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(workerID int) {
			defer wg.Done()
			var local []fileMetrics
			for fw := range jobs {
				if fm, ok := processFile(fw, opts); ok {
					local = append(local, fm)
				}
			}
			results[workerID] = local
		}(w)
	}
	wg.Wait()

	out := &Result{}
	if opts.NeedBase {
		out.Base = aggregateBase(mainPackage, results)
	}
	if opts.NeedComplexity {
		out.Complexity = aggregateComplexity(results, opts.TopN)
	}
	return out, nil
}

func processFile(fw fileWork, opts Options) (fileMetrics, bool) {
	data, err := os.ReadFile(fw.absPath)
	if err != nil {
		return fileMetrics{}, false
	}

	parseMode := parser.Mode(0)
	if opts.NeedBase {
		parseMode = parser.ParseComments
	}

	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, fw.fileName, data, parseMode)
	if err != nil {
		return fileMetrics{}, false
	}

	fm := fileMetrics{work: fw}

	if opts.NeedBase {
		fm.loc = countLinesBytes(data)
		if fw.isTest {
			for _, decl := range astFile.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if ok && fn.Recv == nil && isTestFuncName(fn.Name.Name) {
					fm.testFunctions++
				}
			}
			return fm, true
		}

		for _, cg := range astFile.Comments {
			fm.commentLines += len(cg.List)
		}
		for _, decl := range astFile.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Recv != nil {
					fm.counts.methods++
					if isExported(d.Name) {
						fm.counts.publicMethods++
					}
				} else {
					fm.counts.functions++
					if isExported(d.Name) {
						fm.counts.publicFunctions++
					}
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						switch tx := s.Type.(type) {
						case *ast.StructType:
							fm.counts.structs++
							if isExported(s.Name) {
								fm.counts.publicStructs++
							}
						case *ast.InterfaceType:
							mCount := 0
							if tx.Methods != nil {
								mCount = len(tx.Methods.List)
							}
							fm.ifaceInfos = append(fm.ifaceInfos, interfaces.InterfaceInfo{
								Package: fw.pkgPath,
								File:    fw.fileName,
								Name:    s.Name.String(),
								Methods: mCount,
							})
							if mCount > 0 {
								fm.counts.interfaces++
								if isExported(s.Name) {
									fm.counts.publicInterfaces++
								}
							}
						}
					case *ast.ValueSpec:
						for _, ident := range s.Names {
							if d.Tok == token.VAR {
								fm.counts.variables++
								if isExported(ident) {
									fm.counts.publicVariables++
								}
							} else if d.Tok == token.CONST {
								fm.counts.constants++
								if isExported(ident) {
									fm.counts.publicConstants++
								}
							}
						}
					}
				}
			}
		}
	}

	if opts.NeedComplexity && !fw.isTest {
		cStats := gocyclo.AnalyzeASTFile(astFile, fset, nil)
		for _, st := range cStats {
			fm.cycloSum += st.Complexity
			fm.cycloDetails = append(fm.cycloDetails, complexity.Detail{
				Package:    fw.pkgPath,
				File:       fmt.Sprintf("%s:%d", fw.relFile, st.Pos.Line),
				Function:   st.FuncName,
				Complexity: st.Complexity,
			})
		}

		gStats := gocognit.ComplexityStats(astFile, fset, nil)
		for _, st := range gStats {
			fm.cognitSum += st.Complexity
			fm.cognitDetails = append(fm.cognitDetails, complexity.Detail{
				Package:    fw.pkgPath,
				File:       fmt.Sprintf("%s:%d", fw.relFile, st.Pos.Line),
				Function:   st.FuncName,
				Complexity: st.Complexity,
			})
		}
	}

	return fm, true
}

func aggregateBase(mainPackage string, batches [][]fileMetrics) *cache.MetricsBase {
	locByPkgMap := make(map[string]int)
	locFiles := make([]loc.FileLOC, 0)
	testFiles := make([]loc.FileLOC, 0)
	pkgTypesMap := make(map[string]*types.PackageTypes)

	var (
		totalLOC          int
		totalFiles        int
		commentLines      int
		filesWithComments int
		testLOC           int
		testFileCount     int
		testFuncCount     int
		allIfaces         []interfaces.InterfaceInfo
		projTypes         types.ProjectTypes
	)
	projTypes.ProjectPackage = mainPackage

	for _, batch := range batches {
		for _, fm := range batch {
			if fm.work.isTest {
				testFileCount++
				testLOC += fm.loc
				testFuncCount += fm.testFunctions
				testFiles = append(testFiles, loc.FileLOC{
					Package: fm.work.pkgPath,
					File:    fm.work.fileName,
					LOC:     fm.loc,
				})
				continue
			}

			totalFiles++
			totalLOC += fm.loc
			commentLines += fm.commentLines
			if fm.commentLines > 0 {
				filesWithComments++
			}

			locByPkgMap[fm.work.pkgPath] += fm.loc
			locFiles = append(locFiles, loc.FileLOC{
				Package: fm.work.pkgPath,
				File:    fm.work.fileName,
				LOC:     fm.loc,
			})

			pt, ok := pkgTypesMap[fm.work.pkgPath]
			if !ok {
				pt = &types.PackageTypes{Package: fm.work.pkgPath}
				pkgTypesMap[fm.work.pkgPath] = pt
			}
			pt.SourceFiles++
			addTypeCounts(&pt.SourceFiles, pt, fm.counts)

			projTypes.SourceFiles++
			projTypes.Structs += fm.counts.structs
			projTypes.Interfaces += fm.counts.interfaces
			projTypes.Functions += fm.counts.functions
			projTypes.Methods += fm.counts.methods
			projTypes.Variables += fm.counts.variables
			projTypes.Constants += fm.counts.constants
			projTypes.PublicStructs += fm.counts.publicStructs
			projTypes.PublicInterfaces += fm.counts.publicInterfaces
			projTypes.PublicFunctions += fm.counts.publicFunctions
			projTypes.PublicMethods += fm.counts.publicMethods
			projTypes.PublicVariables += fm.counts.publicVariables
			projTypes.PublicConstants += fm.counts.publicConstants

			allIfaces = append(allIfaces, fm.ifaceInfos...)
		}
	}

	locPackages := make([]loc.PackageLOC, 0, len(locByPkgMap))
	for pkg, count := range locByPkgMap {
		locPackages = append(locPackages, loc.PackageLOC{Package: pkg, LOC: count})
	}
	sort.Slice(locPackages, func(i, j int) bool {
		return locPackages[i].Package < locPackages[j].Package
	})
	sort.Slice(locFiles, func(i, j int) bool {
		if locFiles[i].Package != locFiles[j].Package {
			return locFiles[i].Package < locFiles[j].Package
		}
		return locFiles[i].File < locFiles[j].File
	})

	pkgDetails := make([]types.PackageTypes, 0, len(pkgTypesMap))
	for _, pt := range pkgTypesMap {
		pkgDetails = append(pkgDetails, *pt)
	}
	sort.Slice(pkgDetails, func(i, j int) bool {
		return pkgDetails[i].Package < pkgDetails[j].Package
	})
	projTypes.Packages = len(pkgDetails)
	projTypes.PackageDetails = pkgDetails

	ifaceMetrics := buildInterfaceMetrics(allIfaces)

	var commentRatio, filesCommentRatio, testRatio float64
	if totalLOC > 0 {
		commentRatio = round2(100 * float64(commentLines) / float64(totalLOC))
		testRatio = round2(float64(testLOC) / float64(totalLOC))
	}
	if totalFiles > 0 {
		filesCommentRatio = round2(100 * float64(filesWithComments) / float64(totalFiles))
	}

	sort.Slice(testFiles, func(i, j int) bool {
		if testFiles[i].LOC != testFiles[j].LOC {
			return testFiles[i].LOC > testFiles[j].LOC
		}
		if testFiles[i].Package != testFiles[j].Package {
			return testFiles[i].Package < testFiles[j].Package
		}
		return testFiles[i].File < testFiles[j].File
	})
	if len(testFiles) > 5 {
		testFiles = testFiles[:5]
	}

	return &cache.MetricsBase{
		LOC: &loc.ProjectLOC{
			Total:    totalLOC,
			Packages: locPackages,
			Files:    locFiles,
		},
		Types:      &projTypes,
		Interfaces: ifaceMetrics,
		Comments: &comments.CommentsMetrics{
			TotalLines:             commentLines,
			CommentsRatio:          commentRatio,
			FilesWithComments:      filesWithComments,
			FilesWithCommentsRatio: filesCommentRatio,
		},
		Tests: &cache.TestMetrics{
			LOC:          testLOC,
			Files:        testFileCount,
			Functions:    testFuncCount,
			Ratio:        testRatio,
			LargestFiles: testFiles,
		},
	}
}

func isTestFuncName(name string) bool {
	return strings.HasPrefix(name, "Test") ||
		strings.HasPrefix(name, "Benchmark") ||
		strings.HasPrefix(name, "Fuzz") ||
		strings.HasPrefix(name, "Example")
}

func addTypeCounts(_ *int, pt *types.PackageTypes, c typesCounts) {
	pt.Structs += c.structs
	pt.Interfaces += c.interfaces
	pt.Functions += c.functions
	pt.Methods += c.methods
	pt.Variables += c.variables
	pt.Constants += c.constants
	pt.PublicStructs += c.publicStructs
	pt.PublicInterfaces += c.publicInterfaces
	pt.PublicFunctions += c.publicFunctions
	pt.PublicMethods += c.publicMethods
	pt.PublicVariables += c.publicVariables
	pt.PublicConstants += c.publicConstants
}

func buildInterfaceMetrics(all []interfaces.InterfaceInfo) *interfaces.InterfaceMetrics {
	if len(all) == 0 {
		return &interfaces.InterfaceMetrics{
			AverageMethods:      0,
			InterfaceMaxMethods: []interfaces.InterfaceInfo{},
			InterfaceMinMethods: []interfaces.InterfaceInfo{},
		}
	}

	sort.Slice(all, func(i, j int) bool {
		if all[i].Package != all[j].Package {
			return all[i].Package < all[j].Package
		}
		if all[i].File != all[j].File {
			return all[i].File < all[j].File
		}
		return all[i].Name < all[j].Name
	})

	sum := 0
	minM := math.MaxInt
	maxM := 0
	for _, info := range all {
		sum += info.Methods
		if info.Methods < minM {
			minM = info.Methods
		}
		if info.Methods > maxM {
			maxM = info.Methods
		}
	}

	var minList, maxList []interfaces.InterfaceInfo
	for _, info := range all {
		if info.Methods == minM {
			minList = append(minList, info)
		}
		if info.Methods == maxM {
			maxList = append(maxList, info)
		}
	}

	return &interfaces.InterfaceMetrics{
		AverageMethods:      float64(sum) / float64(len(all)),
		InterfaceMaxMethods: maxList,
		InterfaceMinMethods: minList,
	}
}

func aggregateComplexity(batches [][]fileMetrics, topN int) *complexity.Report {
	var cycloDetails, cognitDetails []complexity.Detail
	var cycloSum, cognitSum int

	for _, batch := range batches {
		for _, fm := range batch {
			cycloSum += fm.cycloSum
			cognitSum += fm.cognitSum
			cycloDetails = append(cycloDetails, fm.cycloDetails...)
			cognitDetails = append(cognitDetails, fm.cognitDetails...)
		}
	}

	return &complexity.Report{
		Cyclomatic: complexity.Summarize(cycloDetails, cycloSum, topN),
		Cognitive:  complexity.Summarize(cognitDetails, cognitSum, topN),
	}
}

func isExported(id *ast.Ident) bool {
	if id == nil || id.Name == "" {
		return false
	}
	for _, r := range id.Name {
		return unicode.IsUpper(r)
	}
	return false
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
