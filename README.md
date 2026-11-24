# Go-Architect

<div align="center">
<img src="./.github/logo.png" width="260" alt="Go-Architect" />
</div>

<p align="center">
  <a href="https://github.com/dundee/go-architect/actions/workflows/build.yml"><img src="https://github.com/dundee/go-architect/actions/workflows/build.yml/badge.svg" alt="CI" /></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-1.27+-00ADD8?logo=go&logoColor=white" alt="Go Version" /></a>
  <a href="./LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License: MIT" /></a>
</p>

Go-Architect is an interactive Software Architecture Analysis Tool for Go Projects. Run a single binary from the command line against any local Go module to explore architectural metrics, dependency structures, complexity, and Git-backed trends in your browser.

---

## Features
- **Abstractness & Instability Chart**: Interactive scatter plot with Main Sequence, Zone of Pain, Zone of Uselessness, overlapping package grouping, hover tooltips, and synced table filtering.
- **Dependency Graph**: Directed graph across Package Layers (`internal`, `organization`, `external`, `standard`) with neighbor inspection and direct jump to Coupling.
- **Dependency Structure Matrix (DSM)**: Package-to-package and 4×4 layer-aggregated matrices with weighted/boolean modes and mutual Dependency Cycle detection.
- **Dependency Coupling**: Inspect how deeply your Project depends on any imported package down to highlighted Go source lines.
- **Project Metrics**: Lines of code, declaration/visibility counts, interface sizing, comment ratios, and built-in Cyclomatic/Cognitive complexity (in-process, zero external binaries required).
- **Trends**: Historical metric trends sampled from Git commits/tags with local per-commit caching and live progress streaming.
- **VCS Analysis**: Contributor activity and Bus Factor Risk across the Repository over configurable time windows.

## Screenshots

### Abstractness & Instability Chart
![Abstractness & Instability Chart](./.github/screenshots-01.png)

### Dependency Graph
![Dependency Graph](./.github/screenshots-02.png)

### Historical Metric Trends
![Historical Metric Trends](./.github/screenshots-03.png)

## Installation

### Requirements
- Go 1.27+

### Install with Go
```bash
go install github.com/dundee/go-architect@latest
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
go-architect .

# Or run directly from source
go run . .

# Specify organization package prefixes and custom listen address
go-architect -path /path/to/repo -org github.com/my-org/ -addr 127.0.0.1:8080

# Start without auto-opening the browser
go-architect -open=false /path/to/repo
```

## Development
```bash
# 1. Start the Go API server
go run . -open=false -path .

# 2. Start Vite dev server (proxies /api to 127.0.0.1:8080)
cd web && npm install && npm run dev

# 3. Rebuild embedded frontend assets
make web-build

# 4. Run Go tests and vet
make test
make vet
```

## Acknowledgements

Originally created by [Francisco Daines](https://github.com/go-architect/go-architect) as a Wails desktop application; reworked into a CLI + embedded React/TypeScript web UI with expanded interactive visualizations and Git-computed historical trends.
