# Compute Historical Metric Trends from Git Trees with Local Commit Cache

Status: accepted (updated from worktree checkout to in-memory tree analysis)

We need historical metric trends for any Go Project without requiring users to commit tracking files into the target Repository or wait weeks to accumulate manual snapshots. Originally we checked out commits into temporary `git worktree` directories and invoked `go-architect-lib`, which ran `go list ./...` (`packages.Load`) 5 times per commit and made larger repositories very slow. Because the historical trend metrics (LOC, comments, declarations, interfaces, internal package Instability, and optional AST complexity) only require `go.mod`'s module path and local `.go` syntax, we now read commit blobs directly from `go-git` trees in memory, parse each `.go` file once in parallel, and store results in a local Protobuf OS cache keyed by commit SHA (with complexity stored in separate `.pb` files).

## Considered Options

- **Repo-tracked snapshot file (`.goarchitect/history.json`)**: Easy to share in Git, but yields zero history on first run and modifies files inside the analyzed Repository.
- **`git worktree` + `go-architect-lib` (`packages.Load`)**: Reuses library functions directly, but incurs disk checkouts, repeated `go list` calls, and failures on commits with un-tidied `go.mod` dependencies.
- **In-memory `go-git` trees + single-pass AST analysis + Protobuf OS cache (chosen)**: Gives instant retroactive trends on day 1 with zero `go mod` / `go list` / disk checkout overhead while keeping the analyzed Repository untouched.
