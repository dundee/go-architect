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
	Mode              SamplingMode     `json:"mode"`
	Months            int              `json:"months"`
	IncludeComplexity bool             `json:"include_complexity"`
	Points            []CommitSnapshot `json:"points"`
	CachedCount       int              `json:"cached_count"`
	ComputedCount     int              `json:"computed_count"`
	SkippedCount      int              `json:"skipped_count"`
	SkippedReasons    []string         `json:"skipped_reasons,omitempty"`
	DurationMs        int64            `json:"duration_ms"`
}
