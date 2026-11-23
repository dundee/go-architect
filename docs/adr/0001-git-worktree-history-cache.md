# Compute Historical Metric Trends via Git Worktrees and Local Commit Cache

Status: accepted

We need historical metric trends for any Go Project without requiring users to commit tracking files into the target Repository or wait weeks to accumulate manual snapshots. Because the underlying Go AST analysis libraries load packages from a directory on disk, we sample past commits (time-bucketed or by Git tag), check each uncached commit out into a temporary `git worktree`, measure its metrics, and cache each `Metric Snapshot` locally in the OS user cache directory keyed by commit SHA.

## Considered Options

- **Repo-tracked snapshot file (`.goarchitect/history.json`)**: Easy to share in Git, but yields zero history on first run and modifies files inside the analyzed Repository.
- **Local user DB (`~/.goarchitect/db/`) with manual saves**: Keeps the Repository clean, but starts empty until snapshots are saved manually over time.
- **Git worktrees + per-commit OS cache (chosen)**: Gives instant retroactive trends on day 1 while keeping the analyzed Repository untouched.

## Consequences

- Historical trends require the Project to live inside a Git Repository; unbuildable historical commits (e.g., missing `go.mod` or unresolvable dependencies) are skipped and surfaced as a count rather than failing the series.
- Cache entries are keyed by commit SHA and schema version, so clean `HEAD` commits or repeated queries return immediately without spawning worktrees, while dirty working trees never poison the cache.
