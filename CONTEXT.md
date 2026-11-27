# Go Architect

Go Architect analyzes the structure, coupling, complexity, and contributor history of Go codebases to support architectural decisions and refactoring.

## Language

### Core Concepts

**Project**:
The target Go module being analyzed, identified by its root directory and module path (`go.mod`).
_Avoid_: Workspace, App, Codebase, Target Module

**Repository**:
The optional Git repository containing the Project, used only when inspecting Git metadata, commit history, or contributor activity.
_Avoid_: Project (when referring specifically to version-control state)

**Package Layer**:
A classification of a Go package relative to the Project: `internal` (belongs to the Project's module), `organization` (matches configured organization prefixes), `external` (third-party dependencies), or `standard` (Go standard library).

### Instability & Coupling

**Main Sequence**:
The ideal balance line where Abstractness plus Instability equals 1 (`A + I = 1`).

**Distance from Main Sequence (D)**:
How far a package lies from the Main Sequence (`|A + I - 1|`), ranging from `0.0` (balanced) to `1.0` (maximum drift).

**Zone of Pain**:
Packages that are both concrete and heavily depended upon (`D >= 0.4` and `A + I < 1`), making changes rigid and costly.

**Zone of Uselessness**:
Packages that are abstract yet unreferenced (`D >= 0.4` and `A + I > 1`), indicating dead or over-engineered abstractions.

### Dependencies & Architecture

**Dependency Cycle**:
A mutual dependency where two packages (or Package Layers) depend on each other directly (`A → B` and `B → A`).

**Dependency Coupling**:
The degree to which the Project's packages and files reference a specific target dependency, down to individual source lines.

**Propagation Cost**:
The percentage of ordered internal package pairs (`u ≠ v`) connected directly or transitively in the internal dependency graph, indicating how widely changes can ripple across the Project.

**Max Dependency Depth**:
The longest directed dependency chain across the condensation DAG of strongly connected components (SCCs) in the internal package graph.

### Code & Test Health

**Exported API Doc Coverage**:
The percentage of exported production types, functions, and methods on exported receivers that have Go doc comments.

**Production vs. Test Packages**:
Production metrics and architectural graphs exclude `*_test.go` files (and generated `// Code generated ... DO NOT EDIT.` files in code/complexity metrics) by default; Test Metrics track test LOC, test functions, and the ratio of production packages that have corresponding `*_test.go` files.

### Repository & Team Health

**Bus Factor Risk**:
The maintainability risk caused by too few contributors modifying a path or file within the Repository's selected time window: `High` (1 contributor), `Moderate` (fewer than 3 contributors when the Repository has more than 4), or `Healthy`.

**Hotspots**:
Production Go files ranked by a combined score of Repository churn (commit frequency and modified lines) and AST complexity (Cyclomatic and Cognitive complexity) to highlight high-risk refactoring candidates.

### Historical Trends

**Metric Snapshot**:
The set of architectural and code metrics (size, declarations, comments, doc coverage, test coverage, Instability health, Propagation Cost, Max Dependency Depth, and Cyclomatic/Cognitive complexity) measured for the Project at a specific Git commit.

**Trend Series**:
A chronological sequence of Metric Snapshots sampled from the Repository's history (by time buckets or Git tags) to reveal how the Project's structure evolves.
