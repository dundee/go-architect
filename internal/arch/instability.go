package arch

import (
	"math"

	"github.com/go-architect/go-architect-lib/instability"
	"github.com/go-architect/go-architect-lib/project"
)

func GetInstability(proj *project.ProjectInfo) ([]*instability.PackageInstability, error) {
	pkgs, err := scanPackages(proj.Path, proj.Package, false)
	if err != nil {
		return nil, err
	}

	efferent := make(map[string]map[string]struct{}, len(pkgs))
	afferent := make(map[string]map[string]struct{}, len(pkgs))
	for _, pkg := range pkgs {
		efferent[pkg.pkgPath] = make(map[string]struct{})
		afferent[pkg.pkgPath] = make(map[string]struct{})
	}

	for _, pkg := range pkgs {
		for _, imp := range pkg.imports {
			if isInternalPackage(imp, proj.Package) {
				efferent[pkg.pkgPath][imp] = struct{}{}
				if _, ok := afferent[imp]; !ok {
					afferent[imp] = make(map[string]struct{})
				}
				afferent[imp][pkg.pkgPath] = struct{}{}
			}
		}
	}

	out := make([]*instability.PackageInstability, 0, len(pkgs))
	for _, pkg := range pkgs {
		ce := len(efferent[pkg.pkgPath])
		ca := len(afferent[pkg.pkgPath])

		var instab, abstr float64
		if ca+ce > 0 {
			instab = round2(float64(ce) / float64(ca+ce))
		}
		if pkg.implementations+pkg.abstractions > 0 {
			abstr = round2(float64(pkg.abstractions) / float64(pkg.implementations+pkg.abstractions))
		}
		dist := round2(math.Abs(instab + abstr - 1))

		out = append(out, &instability.PackageInstability{
			PackageName:          pkg.pkgPath,
			AbstractionsCount:    pkg.abstractions,
			ImplementationsCount: pkg.implementations,
			AfferentCoupling:     ca,
			EfferentCoupling:     ce,
			Instability:          instab,
			Abstractness:         abstr,
			Distance:             dist,
		})
	}

	return out, nil
}
