package trends

import (
	"fmt"
	"time"

	"github.com/dundee/go-architect/internal/cache"
	"github.com/dundee/go-architect/internal/cache/cachepb"
	"github.com/dundee/go-architect/internal/complexity"
)

type Cache struct {
	store *cache.Store
}

func DefaultCacheDir() (string, error) {
	return cache.DefaultDir()
}

func NewCache(dir string) *Cache {
	if dir == "" {
		return nil
	}
	return &Cache{store: cache.NewStore(dir)}
}

func NewCacheFromStore(store *cache.Store) *Cache {
	if store == nil {
		return nil
	}
	return &Cache{store: store}
}

const snapshotSchemaVersion int32 = 3

func (c *Cache) GetSnapshot(projectKey, sha string) (*CommitSnapshot, bool) {
	if c == nil || c.store == nil {
		return nil, false
	}
	var pb cachepb.CommitSnapshotProto
	if !c.store.GetProto("trends", fmt.Sprintf("%s@%s", projectKey, sha), &pb) {
		return nil, false
	}
	m := pb.GetMetrics()
	if m == nil || m.GetSchemaVersion() < snapshotSchemaVersion {
		return nil, false
	}
	return &CommitSnapshot{
		CommitSHA: pb.GetCommitSha(),
		ShortSHA:  pb.GetShortSha(),
		Date:      time.Unix(0, pb.GetDateUnixNano()).UTC(),
		Message:   pb.GetMessage(),
		Tag:       pb.GetTag(),
		Metrics: SnapshotMetrics{
			LOC:                    int(m.GetLoc()),
			Packages:               int(m.GetPackages()),
			SourceFiles:            int(m.GetSourceFiles()),
			Structs:                int(m.GetStructs()),
			Interfaces:             int(m.GetInterfaces()),
			Functions:              int(m.GetFunctions()),
			Methods:                int(m.GetMethods()),
			CommentsTotalLines:     int(m.GetCommentsTotalLines()),
			CommentsRatio:          m.GetCommentsRatio(),
			InterfaceAvgMethods:    m.GetInterfaceAvgMethods(),
			AvgDistance:            m.GetAvgDistance(),
			ZoneOfPainCount:        int(m.GetZoneOfPainCount()),
			ZoneOfUselessnessCount: int(m.GetZoneOfUselessnessCount()),
			TestLOC:                int(m.GetTestLoc()),
			TestFiles:              int(m.GetTestFiles()),
			TestFunctions:          int(m.GetTestFunctions()),
			AvgFunctionLOC:         m.GetAvgFunctionLoc(),
			AvgFunctionParams:      m.GetAvgFunctionParams(),
			APIDocCoverage:         m.GetApiDocCoverage(),
		},
	}, true
}

func (c *Cache) PutSnapshot(projectKey string, snap CommitSnapshot) error {
	if c == nil || c.store == nil {
		return nil
	}
	pb := &cachepb.CommitSnapshotProto{
		CommitSha:    snap.CommitSHA,
		ShortSha:     snap.ShortSHA,
		DateUnixNano: snap.Date.UnixNano(),
		Message:      snap.Message,
		Tag:          snap.Tag,
		Metrics: &cachepb.SnapshotMetricsProto{
			Loc:                    int32(snap.Metrics.LOC),
			Packages:               int32(snap.Metrics.Packages),
			SourceFiles:            int32(snap.Metrics.SourceFiles),
			Structs:                int32(snap.Metrics.Structs),
			Interfaces:             int32(snap.Metrics.Interfaces),
			Functions:              int32(snap.Metrics.Functions),
			Methods:                int32(snap.Metrics.Methods),
			CommentsTotalLines:     int32(snap.Metrics.CommentsTotalLines),
			CommentsRatio:          snap.Metrics.CommentsRatio,
			InterfaceAvgMethods:    snap.Metrics.InterfaceAvgMethods,
			AvgDistance:            snap.Metrics.AvgDistance,
			ZoneOfPainCount:        int32(snap.Metrics.ZoneOfPainCount),
			ZoneOfUselessnessCount: int32(snap.Metrics.ZoneOfUselessnessCount),
			TestLoc:                int32(snap.Metrics.TestLOC),
			TestFiles:              int32(snap.Metrics.TestFiles),
			TestFunctions:          int32(snap.Metrics.TestFunctions),
			AvgFunctionLoc:         snap.Metrics.AvgFunctionLOC,
			AvgFunctionParams:      snap.Metrics.AvgFunctionParams,
			ApiDocCoverage:         snap.Metrics.APIDocCoverage,
			SchemaVersion:          snapshotSchemaVersion,
		},
	}
	return c.store.PutProto("trends", fmt.Sprintf("%s@%s", projectKey, snap.CommitSHA), pb)
}

func (c *Cache) GetComplexity(projectKey, sha string) (*complexity.Report, bool) {
	if c == nil || c.store == nil {
		return nil, false
	}
	return c.store.GetComplexity(projectKey, sha)
}

func (c *Cache) PutComplexity(projectKey, sha string, report *complexity.Report) error {
	if c == nil || c.store == nil {
		return nil
	}
	return c.store.PutComplexity(projectKey, sha, report)
}
