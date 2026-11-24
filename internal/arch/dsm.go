package arch

import (
	"sort"

	"github.com/go-architect/go-architect-lib/dsm"
	"github.com/go-architect/go-architect-lib/project"
)

type dsmDepDetails struct {
	packageName  string
	packageRank  int64
	dependencies int64
	dependants   int64
}

func GetDSM(proj *project.ProjectInfo) (*dsm.DependencyStructureMatrix, error) {
	pkgs, err := scanPackages(proj.Path, proj.Package, true)
	if err != nil {
		return nil, err
	}

	pkgSet := make(map[string]struct{}, len(pkgs)*2)
	for _, p := range pkgs {
		pkgSet[p.pkgPath] = struct{}{}
		for _, imp := range p.imports {
			pkgSet[imp] = struct{}{}
		}
	}

	allPackages := sortedKeys(pkgSet)
	n := len(allPackages)
	indexOf := make(map[string]int, n)
	for idx, p := range allPackages {
		indexOf[p] = idx
	}

	matrix := make([][]int64, n)
	for i := 0; i < n; i++ {
		matrix[i] = make([]int64, n)
	}

	for _, p := range pkgs {
		col := indexOf[p.pkgPath]
		for imp, w := range p.importWeights {
			if row, ok := indexOf[imp]; ok {
				matrix[row][col] += w
			}
		}
	}

	initial := dsm.DependencyStructureMatrix{
		Module:       proj.Package,
		Packages:     allPackages,
		Dependencies: matrix,
	}

	orderedPackages := sortDSM(initial, proj, nil, nil)
	permuted := permuteDSM(allPackages, matrix, orderedPackages)

	return &dsm.DependencyStructureMatrix{
		Module:       proj.Package,
		Packages:     orderedPackages,
		Dependencies: permuted,
	}, nil
}

func permuteDSM(oldPackages []string, oldMatrix [][]int64, newOrder []string) [][]int64 {
	n := len(newOrder)
	oldIndex := make(map[string]int, len(oldPackages))
	for i, p := range oldPackages {
		oldIndex[p] = i
	}

	out := make([][]int64, n)
	for i := 0; i < n; i++ {
		out[i] = make([]int64, n)
		oi := oldIndex[newOrder[i]]
		for j := 0; j < n; j++ {
			oj := oldIndex[newOrder[j]]
			out[i][j] = oldMatrix[oi][oj]
		}
	}
	return out
}

func sortDSM(mat dsm.DependencyStructureMatrix, prj *project.ProjectInfo, head, tail []string) []string {
	if len(mat.Dependencies) == 0 {
		out := make([]string, 0, len(head)+len(tail))
		out = append(out, head...)
		out = append(out, tail...)
		return out
	}
	if len(mat.Dependencies) == 1 {
		out := make([]string, 0, len(head)+len(mat.Packages)+len(tail))
		out = append(out, head...)
		out = append(out, mat.Packages...)
		out = append(out, tail...)
		return out
	}

	noDependencies, noDependants := resolveCandidatesColumns(mat, prj)
	if len(noDependencies) == 0 && len(noDependants) == 0 {
		// Remaining cyclic core: append in current deterministic order.
		out := make([]string, 0, len(head)+len(mat.Packages)+len(tail))
		out = append(out, head...)
		out = append(out, mat.Packages...)
		out = append(out, tail...)
		return out
	}

	inHead := make(map[string]struct{}, len(head)+len(noDependants))
	for _, h := range head {
		inHead[h] = struct{}{}
	}
	for _, n := range noDependants {
		if _, ok := inHead[n.packageName]; !ok {
			head = append(head, n.packageName)
			inHead[n.packageName] = struct{}{}
		}
	}

	for _, n := range noDependencies {
		if _, ok := inHead[n.packageName]; !ok {
			tail = append([]string{n.packageName}, tail...)
		}
	}

	next := removeRowsAndColumns(mat, head, tail)
	return sortDSM(next, prj, head, tail)
}

func resolveCandidatesColumns(mat dsm.DependencyStructureMatrix, prj *project.ProjectInfo) ([]dsmDepDetails, []dsmDepDetails) {
	n := len(mat.Packages)
	var noDependencies, noDependants []dsmDepDetails

	for idx, pkg := range mat.Packages {
		var dependencies, dependants int64
		for i := 0; i < n; i++ {
			dependencies += mat.Dependencies[i][idx]
			dependants += mat.Dependencies[idx][i]
		}
		d := dsmDepDetails{
			packageName:  pkg,
			packageRank:  resolvePackageRank(pkg, prj),
			dependencies: dependencies,
			dependants:   dependants,
		}
		if dependencies == 0 {
			noDependencies = append(noDependencies, d)
		}
		if dependants == 0 {
			noDependants = append(noDependants, d)
		}
	}

	sort.Slice(noDependencies, func(i, j int) bool {
		si := noDependencies[i].packageRank * noDependencies[i].dependants
		sj := noDependencies[j].packageRank * noDependencies[j].dependants
		if si != sj {
			return si > sj
		}
		return noDependencies[i].packageName < noDependencies[j].packageName
	})

	sort.Slice(noDependants, func(i, j int) bool {
		si := noDependants[i].packageRank * noDependants[i].dependencies
		sj := noDependants[j].packageRank * noDependants[j].dependencies
		if si != sj {
			return si > sj
		}
		return noDependants[i].packageName < noDependants[j].packageName
	})

	return noDependencies, noDependants
}

func removeRowsAndColumns(mat dsm.DependencyStructureMatrix, head, tail []string) dsm.DependencyStructureMatrix {
	excluded := make(map[string]struct{}, len(head)+len(tail))
	for _, p := range head {
		excluded[p] = struct{}{}
	}
	for _, p := range tail {
		excluded[p] = struct{}{}
	}

	var keepIndices []int
	var newPackages []string
	for idx, p := range mat.Packages {
		if _, skip := excluded[p]; !skip {
			keepIndices = append(keepIndices, idx)
			newPackages = append(newPackages, p)
		}
	}

	m := make([][]int64, len(keepIndices))
	for r, ri := range keepIndices {
		row := make([]int64, len(keepIndices))
		for c, ci := range keepIndices {
			row[c] = mat.Dependencies[ri][ci]
		}
		m[r] = row
	}

	return dsm.DependencyStructureMatrix{
		Module:       mat.Module,
		Packages:     newPackages,
		Dependencies: m,
	}
}

func resolvePackageRank(pkg string, prj *project.ProjectInfo) int64 {
	if isInternalPackage(pkg, prj.Package) {
		return 1
	}
	if isOrganizationPackage(pkg, prj.OrganizationPackages) {
		return 1000
	}
	if isStandardPackage(pkg) {
		return 1000000000
	}
	return 1000000
}
