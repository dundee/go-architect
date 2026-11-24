# Go-Architect
<div align="center">
<img src="./.github/logo.png" width="300" alt="Go-Architect" align=right />
</div>

---
Go-Architect is a Software Architecture Analysis Tool for Go Projects. Run it from the command line against any local Go module to explore architectural metrics and visualizations in your web browser.

## Features
- **Abstractness & Instability Chart**: Interactive scatter plot with Main Sequence, Zone of Pain, Zone of Uselessness, overlapping package grouping, tooltips, and synced table filtering.
- **Dependency Graph**: Directed graph across Package Layers (`internal`, `organization`, `external`, `standard`) with neighbor inspection.
- **Dependency Structure Matrix (DSM)**: Package-to-package and 4×4 layer-aggregated matrices with weighted/boolean modes and mutual Dependency Cycle detection.
- **Dependency Coupling**: Inspect how deeply your Project depends on any imported package down to highlighted source lines.
- **Project Metrics**: Lines of code, declaration/visibility counts, interface sizing, comment ratios, and built-in Cyclomatic/Cognitive complexity.
- **Trends**: Historical metric trends computed from Git commits/tags with local per-commit caching.
- **VCS Analysis**: Contributor activity and Bus Factor Risk across the Repository over configurable time windows.

## Installation

### Requirements
- Go 1.24+

### Install with Go
```bash
go install github.com/dundee/go-architect/cmd/web@latest
```

### Build from Source
```bash
git clone https://github.com/dundee/go-architect
cd go-architect
make build
```

## Usage
```bash
# Analyze the current directory and open the web UI
go run ./cmd/web .

# Specify organization package prefixes and custom listen address
go run ./cmd/web -path /path/to/repo -org github.com/my-org/ -addr 127.0.0.1:8080

# Start without auto-opening the browser
go run ./cmd/web -open=false /path/to/repo
```

## Development
```bash
# 1. Start the Go API server
go run ./cmd/web -open=false -path .

# 2. Start Vite dev server (proxies /api to 127.0.0.1:8080)
cd web && npm install && npm run dev

# 3. Rebuild embedded frontend assets
make web-build

# 4. Run Go tests
make test
```
