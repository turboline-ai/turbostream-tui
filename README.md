# TurboStream Terminal UI

[![License: MPL 2.0](https://img.shields.io/badge/License-MPL_2.0-brightgreen.svg)](https://opensource.org/licenses/MPL-2.0)

A keyboard-driven terminal interface for monitoring and analyzing real-time data streams with AI.

**Backend Required:** This TUI connects to the [TurboStream backend API](https://github.com/turboline-ai/turbostream). You must have the backend running to use this terminal UI.

A terminal client built with Bubble Tea and Lip Gloss that provides real-time feed monitoring and LLM streaming capabilities.

## Features

### Real-Time Data Streaming
- WebSocket connection to backend at `/ws`
- Token-by-token LLM response streaming
- Feed subscription management
- Automatic reconnection handling

### Observability Dashboard
Press `d` to access the comprehensive dashboard showing:

**Stream Health Panel:**
- Connection status and uptime
- Message throughput (rate, bytes/sec)
- Reconnection count
- Message rate sparkline chart

**LLM Context Panel:**
- Events in context (local cache)
- Memory usage tracking
- Context age (oldest item)
- Dropped/evicted message counts
- Cache memory sparkline chart

**Payload Stats Panel:**
- Last/average/max payload sizes
- Size distribution histogram

**LLM/Tokens Panel:**
- Input/output token counts (last request and session totals)
- Time to First Token (TTFT) metrics
- Generation time with sparkline
- Context utilization percentage
- Events in LLM context
- Error tracking

For detailed metric definitions, see [DASHBOARD_METRICS_REVIEW.md](./DASHBOARD_METRICS_REVIEW.md).

### Live Feed Monitoring
- Feed list with connection status indicators
- Real-time data display
- AI analysis streaming
- Context state visualization

## Prerequisites

### Required
- **Go 1.24+**
- **TurboStream Backend** - Get it from [github.com/turboline-ai/turbostream](https://github.com/turboline-ai/turbostream)

### Start the Backend First

```bash
# Clone and start the backend
git clone https://github.com/turboline-ai/turbostream.git
cd turbostream
cp .env.local.example .env.local
# Edit .env.local with your configuration
go run ./cmd/server
```

The backend will start on `http://localhost:7210` by default.

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `TURBOSTREAM_BACKEND_URL` | Backend REST API URL | `http://localhost:7210` |
| `TURBOSTREAM_WEBSOCKET_URL` | WebSocket endpoint | `ws://localhost:7210/ws` |
| `TURBOSTREAM_TOKEN` | Pre-configured JWT token (optional) | None |
| `TURBOSTREAM_EMAIL` | Pre-fill login email (optional) | None |

## Quick Start

### 1. Start the Backend
See [Prerequisites](#prerequisites) above for backend setup.

### 2. Run the TUI

```bash
git clone https://github.com/turboline-ai/turbostream-tui.git
cd turbostream-tui
go mod download   # Install dependencies
go run .          # or: go build && ./turbostream-tui
```

You'll be prompted to login with credentials from your backend.

## Key Bindings

| Key | Action |
|-----|--------|
| `Enter` | Submit login form / Execute action |
| `d` | Toggle dashboard view |
| `q` | Quit application |
| `↑/↓` | Navigate feed list |
| `c` | Reconnect WebSocket |
| `Tab` | Cycle through form inputs |

## Dashboard Panels

The dashboard uses **sparkline charts** to visualize metric trends over 60-second windows:

- **Green sparklines**: Higher values are good (throughput, rate)
- **Red sparklines**: Higher values are bad (latency, memory usage)

Top summary bar shows: WebSocket status, msg/s, KB/s, context items, tokens, and generation time.

## License

This project is licensed under the **Mozilla Public License 2.0 (MPL-2.0)**. See the [LICENSE](./LICENSE) file for details.

## Contributing

We welcome contributions from the community! Before contributing, please:

1. **Fork the repository** and create a feature branch from `main`.
2. **Follow Go conventions** – run `go fmt` and `go vet` before committing.
3. **Write clear commit messages** describing what changed and why.
4. **Test your changes** – ensure the TUI builds and runs correctly with the backend.
5. **Open a pull request** with a clear description of your changes.

### Code Style
- Use `gofmt` for formatting.
- Keep functions focused and well-documented.
- Follow existing patterns in the codebase for consistency.

### Reporting Issues
- Use GitHub Issues to report bugs or request features.
- Include steps to reproduce, expected behavior, and actual behavior.
- Provide Go version and OS information when reporting bugs.
