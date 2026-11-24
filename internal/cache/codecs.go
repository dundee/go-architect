package cache

import (
	"fmt"
	"time"

	"github.com/dundee/go-architect/backend/git"
	"github.com/dundee/go-architect/internal/cache/cachepb"
	"github.com/dundee/go-architect/internal/complexity"
	"github.com/go-architect/go-architect-lib/coupling"
	"github.com/go-architect/go-architect-lib/dependency"
	"github.com/go-architect/go-architect-lib/dsm"
	"github.com/go-architect/go-architect-lib/instability"
	"github.com/go-architect/go-architect-lib/metrics/comments"
	"github.com/go-architect/go-architect-lib/metrics/interfaces"
	"github.com/go-architect/go-architect-lib/metrics/loc"
	"github.com/go-architect/go-architect-lib/metrics/types"
)

type MetricsBase struct {
	LOC        *loc.ProjectLOC
	Types      *types.ProjectTypes
	Interfaces *interfaces.InterfaceMetrics
	Comments   *comments.CommentsMetrics
}

// --- Complexity (stored separately from trends & base metrics) ---

func (s *Store) GetComplexity(projectKey, sha string) (*complexity.Report, bool) {
	var pb cachepb.ComplexityReportProto
	if !s.GetProto("complexity", fmt.Sprintf("%s@%s", projectKey, sha), &pb) {
		return nil, false
	}
	return &complexity.Report{
		Cyclomatic: fromComplexityOutputProto(pb.GetCyclomatic()),
		Cognitive:  fromComplexityOutputProto(pb.GetCognitive()),
	}, true
}

func (s *Store) PutComplexity(projectKey, sha string, report *complexity.Report) error {
	if report == nil {
		return nil
	}
	pb := &cachepb.ComplexityReportProto{
		Cyclomatic: toComplexityOutputProto(report.Cyclomatic),
		Cognitive:  toComplexityOutputProto(report.Cognitive),
	}
	return s.PutProto("complexity", fmt.Sprintf("%s@%s", projectKey, sha), pb)
}

func toComplexityOutputProto(out complexity.Output) *cachepb.ComplexityOutputProto {
	details := make([]*cachepb.ComplexityDetailProto, 0, len(out.ComplexityDetails))
	for _, d := range out.ComplexityDetails {
		details = append(details, &cachepb.ComplexityDetailProto{
			Package:    d.Package,
			File:       d.File,
			Function:   d.Function,
			Complexity: int32(d.Complexity),
		})
	}
	return &cachepb.ComplexityOutputProto{
		AverageComplexity: out.AverageComplexity,
		Details:           details,
	}
}

func fromComplexityOutputProto(pb *cachepb.ComplexityOutputProto) complexity.Output {
	if pb == nil {
		return complexity.Output{ComplexityDetails: []complexity.Detail{}}
	}
	details := make([]complexity.Detail, 0, len(pb.GetDetails()))
	for _, d := range pb.GetDetails() {
		details = append(details, complexity.Detail{
			Package:    d.GetPackage(),
			File:       d.GetFile(),
			Function:   d.GetFunction(),
			Complexity: int(d.GetComplexity()),
		})
	}
	return complexity.Output{
		AverageComplexity: pb.GetAverageComplexity(),
		ComplexityDetails: details,
	}
}

// --- Instability ---

func (s *Store) GetInstability(projectKey, sha string) ([]*instability.PackageInstability, bool) {
	var pb cachepb.InstabilityCacheProto
	if !s.GetProto("instability", fmt.Sprintf("%s@%s", projectKey, sha), &pb) {
		return nil, false
	}
	out := make([]*instability.PackageInstability, 0, len(pb.GetPackages()))
	for _, p := range pb.GetPackages() {
		out = append(out, &instability.PackageInstability{
			PackageName:          p.GetPackageName(),
			AbstractionsCount:    int(p.GetAbstractionsCount()),
			ImplementationsCount: int(p.GetImplementationsCount()),
			AfferentCoupling:     int(p.GetAfferentCoupling()),
			EfferentCoupling:     int(p.GetEfferentCoupling()),
			Instability:          p.GetInstability(),
			Abstractness:         p.GetAbstractness(),
			Distance:             p.GetDistance(),
		})
	}
	return out, true
}

func (s *Store) PutInstability(projectKey, sha string, pkgs []*instability.PackageInstability) error {
	list := make([]*cachepb.PackageInstabilityProto, 0, len(pkgs))
	for _, p := range pkgs {
		list = append(list, &cachepb.PackageInstabilityProto{
			PackageName:          p.PackageName,
			AbstractionsCount:    int32(p.AbstractionsCount),
			ImplementationsCount: int32(p.ImplementationsCount),
			AfferentCoupling:     int32(p.AfferentCoupling),
			EfferentCoupling:     int32(p.EfferentCoupling),
			Instability:          p.Instability,
			Abstractness:         p.Abstractness,
			Distance:             p.Distance,
		})
	}
	return s.PutProto("instability", fmt.Sprintf("%s@%s", projectKey, sha), &cachepb.InstabilityCacheProto{
		Packages: list,
	})
}

// --- Dependency Graph ---

func (s *Store) GetDependencyGraph(projectKey, sha string) (*dependency.ModuleDependencyGraph, bool) {
	var pb cachepb.DependencyGraphCacheProto
	if !s.GetProto("depgraph", fmt.Sprintf("%s@%s", projectKey, sha), &pb) {
		return nil, false
	}
	rels := make(map[string][]string, len(pb.GetRelations()))
	for k, v := range pb.GetRelations() {
		vals := make([]string, len(v.GetValues()))
		copy(vals, v.GetValues())
		rels[k] = vals
	}
	return &dependency.ModuleDependencyGraph{
		Module:       pb.GetModule(),
		Internal:     nonNilStrings(pb.GetInternal()),
		External:     nonNilStrings(pb.GetExternal()),
		Organization: nonNilStrings(pb.GetOrganization()),
		StandardLib:  nonNilStrings(pb.GetStandard()),
		Relations:    rels,
	}, true
}

func (s *Store) PutDependencyGraph(projectKey, sha string, g *dependency.ModuleDependencyGraph) error {
	if g == nil {
		return nil
	}
	rels := make(map[string]*cachepb.StringListProto, len(g.Relations))
	for k, v := range g.Relations {
		rels[k] = &cachepb.StringListProto{Values: v}
	}
	return s.PutProto("depgraph", fmt.Sprintf("%s@%s", projectKey, sha), &cachepb.DependencyGraphCacheProto{
		Module:       g.Module,
		Internal:     g.Internal,
		External:     g.External,
		Organization: g.Organization,
		Standard:     g.StandardLib,
		Relations:    rels,
	})
}

func nonNilStrings(in []string) []string {
	if in == nil {
		return []string{}
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

// --- DSM ---

func (s *Store) GetDSM(projectKey, sha string) (*dsm.DependencyStructureMatrix, bool) {
	var pb cachepb.DSMCacheProto
	if !s.GetProto("dsm", fmt.Sprintf("%s@%s", projectKey, sha), &pb) {
		return nil, false
	}
	deps := make([][]int64, 0, len(pb.GetDependencies()))
	for _, row := range pb.GetDependencies() {
		vals := make([]int64, len(row.GetValues()))
		copy(vals, row.GetValues())
		deps = append(deps, vals)
	}
	return &dsm.DependencyStructureMatrix{
		Module:       pb.GetModule(),
		Packages:     nonNilStrings(pb.GetPackages()),
		Dependencies: deps,
	}, true
}

func (s *Store) PutDSM(projectKey, sha string, matrix *dsm.DependencyStructureMatrix) error {
	if matrix == nil {
		return nil
	}
	rows := make([]*cachepb.Int64ListProto, 0, len(matrix.Dependencies))
	for _, row := range matrix.Dependencies {
		rows = append(rows, &cachepb.Int64ListProto{Values: row})
	}
	return s.PutProto("dsm", fmt.Sprintf("%s@%s", projectKey, sha), &cachepb.DSMCacheProto{
		Module:       matrix.Module,
		Packages:     matrix.Packages,
		Dependencies: rows,
	})
}

// --- Dependency Coupling ---

func (s *Store) GetCoupling(projectKey, sha, dep string) (*coupling.DependencyCoupling, bool) {
	var pb cachepb.CouplingCacheProto
	if !s.GetProto("coupling", fmt.Sprintf("%s@%s::%s", projectKey, sha, dep), &pb) {
		return nil, false
	}
	pkgDetails := make([]*coupling.PackageCoupling, 0, len(pb.GetDetails()))
	for _, p := range pb.GetDetails() {
		fileDetails := make([]*coupling.FileCoupling, 0, len(p.GetDetails()))
		for _, f := range p.GetDetails() {
			lines := make([]int, 0, len(f.GetCouplingLines()))
			for _, l := range f.GetCouplingLines() {
				lines = append(lines, int(l))
			}
			cDetails := make([]coupling.Detail, 0, len(f.GetDetails()))
			for _, d := range f.GetDetails() {
				cDetails = append(cDetails, coupling.Detail{
					Line:     int(d.GetLine()),
					Comments: d.GetDetails(),
				})
			}
			fileDetails = append(fileDetails, &coupling.FileCoupling{
				Package:       f.GetPackage(),
				File:          f.GetFile(),
				FileContent:   f.GetFileContent(),
				Lines:         lines,
				CouplingLevel: int(f.GetCouplingLevel()),
				Details:       cDetails,
			})
		}
		pkgDetails = append(pkgDetails, &coupling.PackageCoupling{
			Package:       p.GetPackage(),
			CouplingLevel: int(p.GetCouplingLevel()),
			FileDetails:   fileDetails,
		})
	}
	return &coupling.DependencyCoupling{
		Dependency:     pb.GetDependency(),
		CouplingLevel:  int(pb.GetCouplingLevel()),
		PackageDetails: pkgDetails,
	}, true
}

func (s *Store) PutCoupling(projectKey, sha, dep string, res *coupling.DependencyCoupling) error {
	if res == nil {
		return nil
	}
	pkgDetails := make([]*cachepb.PackageCouplingProto, 0, len(res.PackageDetails))
	for _, p := range res.PackageDetails {
		fileDetails := make([]*cachepb.FileCouplingProto, 0, len(p.FileDetails))
		for _, f := range p.FileDetails {
			lines := make([]int32, 0, len(f.Lines))
			for _, l := range f.Lines {
				lines = append(lines, int32(l))
			}
			cDetails := make([]*cachepb.CouplingDetailProto, 0, len(f.Details))
			for _, d := range f.Details {
				cDetails = append(cDetails, &cachepb.CouplingDetailProto{
					Line:    int32(d.Line),
					Details: d.Comments,
				})
			}
			fileDetails = append(fileDetails, &cachepb.FileCouplingProto{
				Package:       f.Package,
				File:          f.File,
				FileContent:   f.FileContent,
				CouplingLines: lines,
				CouplingLevel: int32(f.CouplingLevel),
				Details:       cDetails,
			})
		}
		pkgDetails = append(pkgDetails, &cachepb.PackageCouplingProto{
			Package:       p.Package,
			CouplingLevel: int32(p.CouplingLevel),
			Details:       fileDetails,
		})
	}
	return s.PutProto("coupling", fmt.Sprintf("%s@%s::%s", projectKey, sha, dep), &cachepb.CouplingCacheProto{
		Dependency:    res.Dependency,
		CouplingLevel: int32(res.CouplingLevel),
		Details:       pkgDetails,
	})
}

// --- Metrics Base (LOC, Types, Interfaces, Comments) ---

func (s *Store) GetMetricsBase(projectKey, sha string) (*MetricsBase, bool) {
	var pb cachepb.MetricsBaseCacheProto
	if !s.GetProto("metrics", fmt.Sprintf("%s@%s", projectKey, sha), &pb) {
		return nil, false
	}

	locPkgs := make([]loc.PackageLOC, 0, len(pb.GetLocPackages()))
	for _, p := range pb.GetLocPackages() {
		locPkgs = append(locPkgs, loc.PackageLOC{Package: p.GetPackage(), LOC: int(p.GetLoc())})
	}
	locFiles := make([]loc.FileLOC, 0, len(pb.GetLocFiles()))
	for _, f := range pb.GetLocFiles() {
		locFiles = append(locFiles, loc.FileLOC{Package: f.GetPackage(), File: f.GetFile(), LOC: int(f.GetLoc())})
	}

	var pt types.ProjectTypes
	if pbTypes := pb.GetTypes(); pbTypes != nil {
		pt.ProjectPackage = pbTypes.GetProjectPackage()
		pt.Packages = int(pbTypes.GetPackages())
		if tot := pbTypes.GetTotals(); tot != nil {
			pt.SourceFiles = int(tot.GetSourceFiles())
			pt.Structs = int(tot.GetStructs())
			pt.Interfaces = int(tot.GetInterfaces())
			pt.Functions = int(tot.GetFunctions())
			pt.Methods = int(tot.GetMethods())
			pt.Variables = int(tot.GetVariables())
			pt.Constants = int(tot.GetConstants())
			pt.PublicStructs = int(tot.GetPublicStructs())
			pt.PublicInterfaces = int(tot.GetPublicInterfaces())
			pt.PublicFunctions = int(tot.GetPublicFunctions())
			pt.PublicMethods = int(tot.GetPublicMethods())
			pt.PublicVariables = int(tot.GetPublicVariables())
			pt.PublicConstants = int(tot.GetPublicConstants())
		}
		pt.PackageDetails = make([]types.PackageTypes, 0, len(pbTypes.GetDetails()))
		for _, d := range pbTypes.GetDetails() {
			var item types.PackageTypes
			item.Package = d.GetPackage()
			if c := d.GetCounts(); c != nil {
				item.SourceFiles = int(c.GetSourceFiles())
				item.Structs = int(c.GetStructs())
				item.Interfaces = int(c.GetInterfaces())
				item.Functions = int(c.GetFunctions())
				item.Methods = int(c.GetMethods())
				item.Variables = int(c.GetVariables())
				item.Constants = int(c.GetConstants())
				item.PublicStructs = int(c.GetPublicStructs())
				item.PublicInterfaces = int(c.GetPublicInterfaces())
				item.PublicFunctions = int(c.GetPublicFunctions())
				item.PublicMethods = int(c.GetPublicMethods())
				item.PublicVariables = int(c.GetPublicVariables())
				item.PublicConstants = int(c.GetPublicConstants())
			}
			pt.PackageDetails = append(pt.PackageDetails, item)
		}
	}

	ifaceMetrics := &interfaces.InterfaceMetrics{}
	if pbIf := pb.GetInterfaces(); pbIf != nil {
		ifaceMetrics.AverageMethods = pbIf.GetAverageMethods()
		ifaceMetrics.InterfaceMaxMethods = fromInterfaceInfoProtos(pbIf.GetMaxMethods())
		ifaceMetrics.InterfaceMinMethods = fromInterfaceInfoProtos(pbIf.GetMinMethods())
	}

	commMetrics := &comments.CommentsMetrics{}
	if pbComm := pb.GetComments(); pbComm != nil {
		commMetrics.TotalLines = int(pbComm.GetTotalLines())
		commMetrics.CommentsRatio = pbComm.GetRatio()
		commMetrics.FilesWithComments = int(pbComm.GetFilesWithComments())
		commMetrics.FilesWithCommentsRatio = pbComm.GetFilesWithCommentsRatio()
	}

	return &MetricsBase{
		LOC: &loc.ProjectLOC{
			Total:    int(pb.GetLocTotal()),
			Packages: locPkgs,
			Files:    locFiles,
		},
		Types:      &pt,
		Interfaces: ifaceMetrics,
		Comments:   commMetrics,
	}, true
}

func (s *Store) PutMetricsBase(projectKey, sha string, m *MetricsBase) error {
	if m == nil || m.LOC == nil || m.Types == nil || m.Interfaces == nil || m.Comments == nil {
		return nil
	}
	locPkgs := make([]*cachepb.PackageLOCProto, 0, len(m.LOC.Packages))
	for _, p := range m.LOC.Packages {
		locPkgs = append(locPkgs, &cachepb.PackageLOCProto{Package: p.Package, Loc: int32(p.LOC)})
	}
	locFiles := make([]*cachepb.FileLOCProto, 0, len(m.LOC.Files))
	for _, f := range m.LOC.Files {
		locFiles = append(locFiles, &cachepb.FileLOCProto{Package: f.Package, File: f.File, Loc: int32(f.LOC)})
	}

	pkgDetails := make([]*cachepb.PackageTypesProto, 0, len(m.Types.PackageDetails))
	for _, d := range m.Types.PackageDetails {
		pkgDetails = append(pkgDetails, &cachepb.PackageTypesProto{
			Package: d.Package,
			Counts: &cachepb.TypeCounterProto{
				SourceFiles:      int32(d.SourceFiles),
				Structs:          int32(d.Structs),
				Interfaces:       int32(d.Interfaces),
				Functions:        int32(d.Functions),
				Methods:          int32(d.Methods),
				Variables:        int32(d.Variables),
				Constants:        int32(d.Constants),
				PublicStructs:    int32(d.PublicStructs),
				PublicInterfaces: int32(d.PublicInterfaces),
				PublicFunctions:  int32(d.PublicFunctions),
				PublicMethods:    int32(d.PublicMethods),
				PublicVariables:  int32(d.PublicVariables),
				PublicConstants:  int32(d.PublicConstants),
			},
		})
	}

	pb := &cachepb.MetricsBaseCacheProto{
		LocTotal:    int32(m.LOC.Total),
		LocPackages: locPkgs,
		LocFiles:    locFiles,
		Types: &cachepb.ProjectTypesProto{
			ProjectPackage: m.Types.ProjectPackage,
			Packages:       int32(m.Types.Packages),
			Totals: &cachepb.TypeCounterProto{
				SourceFiles:      int32(m.Types.SourceFiles),
				Structs:          int32(m.Types.Structs),
				Interfaces:       int32(m.Types.Interfaces),
				Functions:        int32(m.Types.Functions),
				Methods:          int32(m.Types.Methods),
				Variables:        int32(m.Types.Variables),
				Constants:        int32(m.Types.Constants),
				PublicStructs:    int32(m.Types.PublicStructs),
				PublicInterfaces: int32(m.Types.PublicInterfaces),
				PublicFunctions:  int32(m.Types.PublicFunctions),
				PublicMethods:    int32(m.Types.PublicMethods),
				PublicVariables:  int32(m.Types.PublicVariables),
				PublicConstants:  int32(m.Types.PublicConstants),
			},
			Details: pkgDetails,
		},
		Interfaces: &cachepb.InterfaceMetricsProto{
			AverageMethods: m.Interfaces.AverageMethods,
			MaxMethods:     toInterfaceInfoProtos(m.Interfaces.InterfaceMaxMethods),
			MinMethods:     toInterfaceInfoProtos(m.Interfaces.InterfaceMinMethods),
		},
		Comments: &cachepb.CommentsMetricsProto{
			TotalLines:             int32(m.Comments.TotalLines),
			Ratio:                  m.Comments.CommentsRatio,
			FilesWithComments:      int32(m.Comments.FilesWithComments),
			FilesWithCommentsRatio: m.Comments.FilesWithCommentsRatio,
		},
	}
	return s.PutProto("metrics", fmt.Sprintf("%s@%s", projectKey, sha), pb)
}

func toInterfaceInfoProtos(items []interfaces.InterfaceInfo) []*cachepb.InterfaceInfoProto {
	out := make([]*cachepb.InterfaceInfoProto, 0, len(items))
	for _, item := range items {
		out = append(out, &cachepb.InterfaceInfoProto{
			Package: item.Package,
			File:    item.File,
			Name:    item.Name,
			Methods: int32(item.Methods),
		})
	}
	return out
}

func fromInterfaceInfoProtos(items []*cachepb.InterfaceInfoProto) []interfaces.InterfaceInfo {
	out := make([]interfaces.InterfaceInfo, 0, len(items))
	for _, item := range items {
		out = append(out, interfaces.InterfaceInfo{
			Package: item.GetPackage(),
			File:    item.GetFile(),
			Name:    item.GetName(),
			Methods: int(item.GetMethods()),
		})
	}
	return out
}

// --- VCS ---

func (s *Store) GetVCS(projectKey, sha string, months int, day string) (*git.VCSAnalysisInfo, bool) {
	var pb cachepb.VCSCacheProto
	if !s.GetProto("vcs", fmt.Sprintf("%s@%s::%d::%s", projectKey, sha, months, day), &pb) {
		return nil, false
	}
	since := time.Unix(0, pb.GetSinceUnixNano()).UTC()
	authors := make([]git.AuthorInfo, 0, len(pb.GetAuthorDetails()))
	for _, a := range pb.GetAuthorDetails() {
		authors = append(authors, git.AuthorInfo{
			Name:          a.GetName(),
			Commits:       int(a.GetCommits()),
			ModifiedLines: int(a.GetModifiedLines()),
		})
	}
	return &git.VCSAnalysisInfo{
		Since: &since,
		AuthorsInfo: git.AuthorsInfo{
			TotalAuthors:       int(pb.GetTotalAuthors()),
			TotalCommits:       int(pb.GetTotalCommits()),
			TotalModifications: int(pb.GetTotalModifications()),
			AuthorDetails:      authors,
		},
		ModificationsInfo: git.Modifications{
			PathModifications: fromModificationsProtos(pb.GetPathModifications()),
			FileModifications: fromModificationsProtos(pb.GetFileModifications()),
		},
	}, true
}

func (s *Store) PutVCS(projectKey, sha string, months int, day string, v *git.VCSAnalysisInfo) error {
	if v == nil || v.Since == nil {
		return nil
	}
	authors := make([]*cachepb.AuthorInfoProto, 0, len(v.AuthorsInfo.AuthorDetails))
	for _, a := range v.AuthorsInfo.AuthorDetails {
		authors = append(authors, &cachepb.AuthorInfoProto{
			Name:          a.Name,
			Commits:       int32(a.Commits),
			ModifiedLines: int32(a.ModifiedLines),
		})
	}
	pb := &cachepb.VCSCacheProto{
		SinceUnixNano:      v.Since.UnixNano(),
		TotalAuthors:       int32(v.AuthorsInfo.TotalAuthors),
		TotalCommits:       int32(v.AuthorsInfo.TotalCommits),
		TotalModifications: int32(v.AuthorsInfo.TotalModifications),
		AuthorDetails:      authors,
		PathModifications:  toModificationsProtos(v.ModificationsInfo.PathModifications),
		FileModifications:  toModificationsProtos(v.ModificationsInfo.FileModifications),
	}
	return s.PutProto("vcs", fmt.Sprintf("%s@%s::%d::%s", projectKey, sha, months, day), pb)
}

func toModificationsProtos(items []git.ModificationsInfo) []*cachepb.ModificationsInfoProto {
	out := make([]*cachepb.ModificationsInfoProto, 0, len(items))
	for _, item := range items {
		summaries := make([]*cachepb.ContributorSummaryProto, 0, len(item.AuthorsSummary))
		for _, a := range item.AuthorsSummary {
			summaries = append(summaries, &cachepb.ContributorSummaryProto{
				Name:          a.Name,
				Modifications: int32(a.Modifications),
			})
		}
		out = append(out, &cachepb.ModificationsInfoProto{
			Source:             item.Source,
			TotalModifications: int32(item.TotalModifications),
			AuthorsSummary:     summaries,
		})
	}
	return out
}

func fromModificationsProtos(items []*cachepb.ModificationsInfoProto) []git.ModificationsInfo {
	out := make([]git.ModificationsInfo, 0, len(items))
	for _, item := range items {
		summaries := make([]git.ContributorSummary, 0, len(item.GetAuthorsSummary()))
		for _, a := range item.GetAuthorsSummary() {
			summaries = append(summaries, git.ContributorSummary{
				Name:          a.GetName(),
				Modifications: int(a.GetModifications()),
			})
		}
		out = append(out, git.ModificationsInfo{
			Source:             item.GetSource(),
			TotalModifications: int(item.GetTotalModifications()),
			AuthorsSummary:     summaries,
		})
	}
	return out
}
