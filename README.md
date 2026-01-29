# TurboStream Terminal UI

[![License: MPL 2.0](https://img.shields.io/badge/License-MPL_2.0-brightgreen.svg)](https://opensource.org/licenses/MPL-2.0)

**Keyboard-driven terminal interface for real-time data streaming with AI**

A beautiful terminal UI built with Bubble Tea for monitoring high-velocity data streams and analyzing them with LLMs in real-time.

---

## Quick Start Video

> 📹 **Coming Soon:** Watch our TUI demo and setup guide on YouTube

---

## Get Started in 3 Steps

### 1. Start the Backend

The TUI connects to the [TurboStream backend](https://github.com/turboline-ai/turbostream). Start it first:

```bash
# Clone and run the backend
git clone https://github.com/turboline-ai/turbostream.git
cd turbostream
cp .env.local.example .env.local
# Edit .env.local with your MongoDB and LLM credentials
go run ./cmd/server
```

Backend runs at `http://localhost:7210` by default.

### 2. Clone & Run the TUI

```bash
git clone https://github.com/turboline-ai/turbostream-tui.git
cd turbostream-tui
go run .
```

### 3. Login & Start Monitoring

Enter your credentials (from backend user registration) and start streaming! 🚀

---

## Features

### Real-Time Streaming
- WebSocket connection with automatic reconnection
- Token-by-token LLM response streaming
- Live feed data monitoring
- Sub-second latency

### Observability Dashboard

Press `d` to toggle the dashboard:

> 📹 **Coming Soon:** Watch the dashboard tour video

**Panels:**
- **Stream Health** - Connection status, throughput, reconnections
- **LLM Context** - Memory usage, context size, eviction stats
- **Payload Stats** - Size distribution, averages, histograms
- **LLM Performance** - Token counts, TTFT, generation time

**Visualization:**
- Sparkline charts for real-time metric trends
- 60-second rolling windows
- Color-coded indicators (green = good, red = issues)

For detailed metric definitions: [DASHBOARD_METRICS_REVIEW.md](./DASHBOARD_METRICS_REVIEW.md)

### Feed Management
- Browse available feeds
- Subscribe/unsubscribe in real-time
- Monitor multiple feeds simultaneously
- Custom AI prompts per feed

---

## Key Bindings

| Key | Action |
|-----|--------|
| `Enter` | Submit / Execute action |
| `d` | Toggle dashboard view |
| `q` | Quit application |
| `↑/↓` | Navigate feed list |
| `c` | Reconnect WebSocket |
| `Tab` | Cycle through inputs |
| `Esc` | Go back / Cancel |

> 📹 **Coming Soon:** Watch the keyboard shortcuts tutorial

---

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `TURBOSTREAM_BACKEND_URL` | Backend REST API URL | `http://localhost:7210` |
| `TURBOSTREAM_WEBSOCKET_URL` | WebSocket endpoint | `ws://localhost:7210/ws` |
| `TURBOSTREAM_TOKEN` | Pre-configured JWT token | None |
| `TURBOSTREAM_EMAIL` | Pre-fill login email | None |

**Example:**

```bash
export TURBOSTREAM_BACKEND_URL=https://your-backend.railway.app
export TURBOSTREAM_WEBSOCKET_URL=wss://your-backend.railway.app/ws
go run .
```

---

## Screenshots

### Main Feed View
![Feed View](https://turbocdn.blob.core.windows.net/blog-images/terminal-ui.png)

### Observability Dashboard
> 📹 **Coming Soon:** Dashboard walkthrough video

---

## Development

### Prerequisites
- Go 1.24+
- Running TurboStream backend

### Build from Source

```bash
# Clone
git clone https://github.com/turboline-ai/turbostream-tui.git
cd turbostream-tui

# Install dependencies
go mod download

# Build
go build -o turbostream-tui .

# Run
./turbostream-tui
```

### Project Structure

```
turbostream-tui/
├── main.go              # Main application entry
├── ws.go                # WebSocket handling
├── pkg/
│   └── api/             # Backend API client
└── README.md
```

### Running Tests

```bash
go test ./...
```

---

## Troubleshooting

### Can't Connect to Backend

**Error:** `Connection refused` or `Failed to connect`

**Fix:**
1. Check backend is running: `curl http://localhost:7210/health`
2. Verify `TURBOSTREAM_BACKEND_URL` is correct
3. Check firewall settings

### WebSocket Disconnects

**Error:** `WebSocket connection closed`

**Fix:**
- Press `c` to reconnect manually
- Check backend logs for errors
- Verify network stability

### Authentication Failed

**Error:** `Invalid token` or `Auth failed`

**Fix:**
1. Register a user on the backend first
2. Use correct email/password
3. Check JWT token hasn't expired (7 days)

**Need more help?** [GitHub Issues](https://github.com/turboline-ai/turbostream-tui/issues)

---

## Resources

- 🏠 **TurboStream Backend**: [github.com/turboline-ai/turbostream](https://github.com/turboline-ai/turbostream)
- 📚 **API Documentation**: [turboline.ai/docs/api](https://turboline.ai/docs/api)
- 📖 **Dashboard Metrics**: [DASHBOARD_METRICS_REVIEW.md](./DASHBOARD_METRICS_REVIEW.md)
- 💬 **Discussions**: [GitHub Discussions](https://github.com/turboline-ai/turbostream-tui/discussions)
- 🐛 **Report Issues**: [GitHub Issues](https://github.com/turboline-ai/turbostream-tui/issues)
- 📺 **Video Tutorials**: [YouTube](https://youtube.com/@turboline-ai)

---

## Contributing

We welcome contributions!

**Quick Start:**
1. Fork the repository
2. Create a feature branch: `git checkout -b feature/my-feature`
3. Make your changes
4. Run tests: `go test ./...`
5. Format code: `go fmt ./...`
6. Submit a pull request

**Code Style:**
- Use `gofmt` for formatting
- Follow existing patterns
- Keep functions focused and documented
- Test your changes with the backend

---

## License

Licensed under the **Mozilla Public License 2.0 (MPL-2.0)**.

See [LICENSE](LICENSE) for details.

---

**Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) 🫧**

**Made with ❤️ by [Turboline AI](https://turboline.ai)**

Copyright 2024-2025 Turboline AI. All rights reserved.
