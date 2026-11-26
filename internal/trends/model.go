package trends

import "time"

type SamplingMode string

const (
	ModeBuckets SamplingMode = "buckets"
	ModeTags    SamplingMode = "tags"
)

type SnapshotMetrics struct {
	LOC                     int      `json:"loc"`
	Packages                int      `json:"packages"`
	SourceFiles             int      `json:"source_files"`
	Structs                 int      `json:"structs"`
	Interfaces              int      `json:"interfaces"`
	Functions               int      `json:"functions"`
	Methods                 int      `json:"methods"`
	CommentsTotalLines      int      `json:"comments_total_lines"`
	CommentsRatio           float64  `json:"comments_ratio"`
	InterfaceAvgMethods     float64  `json:"interface_avg_methods"`
	AvgDistance             float64  `json:"avg_distance"`
	ZoneOfPainCount         int      `json:"zone_of_pain_count"`
	ZoneOfUselessnessCount  int      `json:"zone_of_uselessness_count"`
	TestLOC                 int      `json:"test_loc"`
	TestFiles               int      `json:"test_files"`
	TestFunctions           int      `json:"test_functions"`
	TestedPackages          int      `json:"tested_packages"`
	UntestedPackages        int      `json:"untested_packages"`
	TestedPackagesRatio     float64  `json:"tested_packages_ratio"`
	AvgFunctionLOC          float64  `json:"avg_function_loc"`
	AvgFunctionParams       float64  `json:"avg_function_params"`
	APIDocCoverage          float64  `json:"api_doc_coverage"`
	CyclomaticComplexityAvg *float64 `json:"cyclomatic_complexity_avg,omitempty"`
	CognitiveComplexityAvg  *float64 `json:"cognitive_complexity_avg,omitempty"`
}

type CommitSnapshot struct {
	CommitSHA string          `json:"commit_sha"`
	ShortSHA  string          `json:"short_sha"`
	Date      time.Time       `json:"date"`
	Message   string          `json:"message"`
	Tag       string          `json:"tag,omitempty"`
	Metrics   SnapshotMetrics `json:"metrics"`
	Cached    bool            `json:"cached"`
}

type TrendsReport struct {
	Mode           SamplingMode     `json:"mode"`
	Months         int              `json:"months"`
	Points         []CommitSnapshot `json:"points"`
	CachedCount    int              `json:"cached_count"`
	ComputedCount  int              `json:"computed_count"`
	SkippedCount   int              `json:"skipped_count"`
	SkippedReasons []string         `json:"skipped_reasons,omitempty"`
	DurationMs     int64            `json:"duration_ms"`
}

type ProgressEvent struct {
	Type      string          `json:"type"` // "start" | "progress"
	Index     int             `json:"index,omitempty"`
	Total     int             `json:"total"`
	CommitSHA string          `json:"commit_sha,omitempty"`
	ShortSHA  string          `json:"short_sha,omitempty"`
	Message   string          `json:"message,omitempty"`
	Tag       string          `json:"tag,omitempty"`
	Status    string          `json:"status,omitempty"` // "cached" | "computing" | "done" | "skipped"
	Reason    string          `json:"reason,omitempty"`
	Snapshot  *CommitSnapshot `json:"snapshot,omitempty"`
}
