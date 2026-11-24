package arch

import (
	"sort"

	"github.com/go-architect/go-architect-lib/dependency"
	"github.com/go-architect/go-architect-lib/project"
)

func GetDependencyGraph(proj *project.ProjectInfo) (*dependency.ModuleDependencyGraph, error) {
	pkgs, err := scanPackages(proj.Path, proj.Package, false)
	if err != nil {
		return nil, err
	}

	internalSet := make(map[string]struct{}, len(pkgs))
	orgSet := make(map[string]struct{})
	extSet := make(map[string]struct{})
	stdSet := make(map[string]struct{})
	relations := make(map[string][]string, len(pkgs))

	for _, pkg := range pkgs {
		internalSet[pkg.pkgPath] = struct{}{}
		rels := make([]string, len(pkg.imports))
		copy(rels, pkg.imports)
		relations[pkg.pkgPath] = rels

		for _, imp := range pkg.imports {
			if isInternalPackage(imp, proj.Package) {
				continue
			}
			if isStandardPackage(imp) {
				stdSet[imp] = struct{}{}
			} else if isOrganizationPackage(imp, proj.OrganizationPackages) {
				orgSet[imp] = struct{}{}
			} else {
				extSet[imp] = struct{}{}
			}
		}
	}

	return &dependency.ModuleDependencyGraph{
		Module:       proj.Package,
		Internal:     sortedKeys(internalSet),
		Organization: sortedKeys(orgSet),
		External:     sortedKeys(extSet),
		StandardLib:  sortedKeys(stdSet),
		Relations:    relations,
	}, nil
}

func sortedKeys(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
