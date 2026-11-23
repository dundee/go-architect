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

### Repository & Team Health

**Bus Factor Risk**:
The maintainability risk caused by too few contributors modifying a path or file within the Repository's selected time window: `High` (1 contributor), `Moderate` (fewer than 3 contributors when the Repository has more than 4), or `Healthy`.

### Historical Trends

**Metric Snapshot**:
The set of architectural and code metrics (size, declarations, comments, Instability health, and optional complexity) measured for the Project at a specific Git commit.

**Trend Series**:
A chronological sequence of Metric Snapshots sampled from the Repository's history (by time buckets or Git tags) to reveal how the Project's structure evolves.
