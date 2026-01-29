package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/turboline-ai/turbostream-tui/internal/metrics"
)

// RenderDashboardView renders the complete observability dashboard for a feed
func RenderDashboardView(dm metrics.DashboardMetrics, termWidth, termHeight int) string {
	if len(dm.Feeds) == 0 {
		return renderNoFeeds(termWidth)
	}

	// Ensure selected index is valid
	if dm.SelectedIdx < 0 || dm.SelectedIdx >= len(dm.Feeds) {
		dm.SelectedIdx = 0
	}

	fm := dm.Feeds[dm.SelectedIdx]

	// Sidebar width for feed list
	sidebarWidth := 22
	contentWidth := termWidth - sidebarWidth - 3

	sidebar := renderFeedSidebar(dm, sidebarWidth, termHeight-10)

	// Build main content area
	var contentBuilder strings.Builder

	// Header
	statusIcon := "●"
	statusStyle := GoodValueStyle
	if !fm.WSConnected {
		statusStyle = BadValueStyle
	}
	title := fmt.Sprintf("%s  %s", fm.Name, statusStyle.Render(statusIcon))
	contentBuilder.WriteString(lipgloss.NewStyle().Bold(true).Foreground(CyanColor).Render(title))
	contentBuilder.WriteString("\n")

	// Summary bar
	contentBuilder.WriteString(renderSummaryBar(fm, contentWidth))
	contentBuilder.WriteString("\n")

	// Calculate panel widths
	panelWidth := (contentWidth - 2) / 2
	if panelWidth < 35 {
		panelWidth = contentWidth - 2
	}

	// Top row: Stream Health | Cache Health
	streamPanel := renderStreamHealthPanel(fm, panelWidth)
	cachePanel := renderCacheHealthPanel(fm, panelWidth)

	if contentWidth >= 72 {
		contentBuilder.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, streamPanel, " ", cachePanel))
	} else {
		contentBuilder.WriteString(streamPanel)
		contentBuilder.WriteString("\n")
		contentBuilder.WriteString(cachePanel)
	}
	contentBuilder.WriteString("\n")

	// Middle row: Payload Histogram | LLM Usage
	payloadPanel := renderPayloadPanel(fm, panelWidth)
	llmPanel := renderLLMPanel(fm, panelWidth)

	if contentWidth >= 72 {
		contentBuilder.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, payloadPanel, " ", llmPanel))
	} else {
		contentBuilder.WriteString(payloadPanel)
		contentBuilder.WriteString("\n")
		contentBuilder.WriteString(llmPanel)
	}

	// Join sidebar and content horizontally
	mainView := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, "  ", contentBuilder.String())

	// Help line
	helpLine := HelpStyle.Render("↑/↓: select feed | Tab: switch tab | q: quit")

	return lipgloss.JoinVertical(lipgloss.Left, mainView, "", helpLine)
}

func renderNoFeeds(width int) string {
	return lipgloss.NewStyle().
		Foreground(DimCyanColor).
		Align(lipgloss.Center).
		Width(width).
		Render("No feeds connected.\n\nSubscribe to a feed to see metrics.")
}

func renderFeedSidebar(dm metrics.DashboardMetrics, width, maxHeight int) string {
	var lines []string

	visibleFeeds := maxHeight - 6
	if visibleFeeds < 3 {
		visibleFeeds = 3
	}

	// Determine scroll window
	startIdx := 0
	endIdx := len(dm.Feeds)

	if len(dm.Feeds) > visibleFeeds {
		halfVisible := visibleFeeds / 2
		startIdx = dm.SelectedIdx - halfVisible
		if startIdx < 0 {
			startIdx = 0
		}
		endIdx = startIdx + visibleFeeds
		if endIdx > len(dm.Feeds) {
			endIdx = len(dm.Feeds)
			startIdx = endIdx - visibleFeeds
			if startIdx < 0 {
				startIdx = 0
			}
		}
	}

	// Show scroll indicator at top if needed
	if startIdx > 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(DimCyanColor).Render("  ▲ more"))
	}

	// Render feed items
	for i := startIdx; i < endIdx; i++ {
		feed := dm.Feeds[i]

		icon := FeedItemDisconnectedIcon()
		if feed.WSConnected {
			icon = FeedItemConnectedIcon()
		}

		name := feed.Name
		maxNameLen := width - 6
		if maxNameLen < 8 {
			maxNameLen = 8
		}
		if len(name) > maxNameLen {
			name = name[:maxNameLen-1] + "…"
		}

		itemText := fmt.Sprintf("%s %s", icon, name)

		if i == dm.SelectedIdx {
			lines = append(lines, FeedItemSelectedStyle.Width(width-4).Render(itemText))
		} else {
			lines = append(lines, FeedItemNormalStyle.Width(width-4).Render(itemText))
		}
	}

	// Show scroll indicator at bottom if needed
	if endIdx < len(dm.Feeds) {
		lines = append(lines, lipgloss.NewStyle().Foreground(DimCyanColor).Render("  ▼ more"))
	}

	// Add feed count at bottom
	lines = append(lines, "")
	countText := fmt.Sprintf("%d/%d", dm.SelectedIdx+1, len(dm.Feeds))
	lines = append(lines, lipgloss.NewStyle().Foreground(GrayColor).Align(lipgloss.Center).Width(width-4).Render(countText))

	content := strings.Join(lines, "\n")
	return RenderPanel("Feeds", content, width)
}

func renderSummaryBar(fm metrics.FeedMetrics, width int) string {
	wsStatus := GoodValueStyle.Render("● Connected")
	if !fm.WSConnected {
		wsStatus = BadValueStyle.Render("● Disconnected")
	}

	msgRate := fmt.Sprintf("%.1f msg/s", fm.MessagesPerSecond10s)
	byteRate := fmt.Sprintf("%.1f KB/s", fm.BytesPerSecond10s/1024)
	cacheInfo := fmt.Sprintf("ctx: %d items", fm.CacheItemsCurrent)
	tokens := fmt.Sprintf("in: %d out: %d", fm.InputTokensLast, fm.OutputTokensLast)
	genTime := fmt.Sprintf("gen: %.0fms", fm.GenerationTimeAvgMs)

	parts := []string{wsStatus, msgRate, byteRate, cacheInfo, tokens, genTime}
	summary := strings.Join(parts, "  │  ")

	return SummaryBarStyle.Width(width - 4).Render(summary)
}

func renderStreamHealthPanel(fm metrics.FeedMetrics, width int) string {
	var lines []string

	connStatus := GoodValueStyle.Render("Connected ✓")
	if !fm.WSConnected {
		connStatus = BadValueStyle.Render("Disconnected ✗")
	}
	lines = append(lines, RenderColoredMetric("Status", connStatus, MetricValueStyle))

	lines = append(lines, RenderMetric("Messages Received", fmt.Sprintf("%d", fm.MessagesReceivedTotal)))
	lines = append(lines, RenderMetric("Rate", fmt.Sprintf("%.1f msg/s", fm.MessagesPerSecond10s)))

	if len(fm.MsgRateHistory) > 0 {
		sparkWidth := width - 12
		if sparkWidth > 40 {
			sparkWidth = 40
		}
		sparkline := RenderSparkline(fm.MsgRateHistory, sparkWidth, false)
		lines = append(lines, MetricLabelStyle.Render("Trend: ")+sparkline)
	}

	lines = append(lines, RenderMetric("Throughput", fmt.Sprintf("%.1f KB/s", fm.BytesPerSecond10s/1024)))
	lines = append(lines, RenderMetric("Total Bytes", HumanizeBytes(fm.BytesReceivedTotal)))

	ageStyle := GoodValueStyle
	if fm.LastMessageAgeSeconds > 30 {
		ageStyle = WarnValueStyle
	}
	if fm.LastMessageAgeSeconds > 60 {
		ageStyle = BadValueStyle
	}
	lines = append(lines, RenderColoredMetric("Last Msg", HumanizeDuration(fm.LastMessageAgeSeconds)+" ago", ageStyle))

	lines = append(lines, RenderMetric("Reconnects", fmt.Sprintf("%d", fm.ReconnectsTotal)))
	lines = append(lines, RenderMetric("Uptime", HumanizeDuration(fm.CurrentUptimeSeconds)))

	return RenderPanel("Stream / WebSocket", strings.Join(lines, "\n"), width)
}

func renderCacheHealthPanel(fm metrics.FeedMetrics, width int) string {
	var lines []string

	lines = append(lines, RenderMetric("Events in Context", fmt.Sprintf("%d", fm.CacheItemsCurrent)))

	memStyle := GoodValueStyle
	if fm.CacheApproxBytes > 50*1024*1024 {
		memStyle = WarnValueStyle
	}
	if fm.CacheApproxBytes > 100*1024*1024 {
		memStyle = BadValueStyle
	}
	lines = append(lines, RenderColoredMetric("Context Size", HumanizeBytes(fm.CacheApproxBytes), memStyle))

	if len(fm.CacheBytesHistory) > 0 {
		sparkWidth := width - 12
		if sparkWidth > 40 {
			sparkWidth = 40
		}
		sparkline := RenderSparkline(fm.CacheBytesHistory, sparkWidth, true)
		lines = append(lines, MetricLabelStyle.Render("Trend: ")+sparkline)
	}

	lines = append(lines, RenderMetric("Context Age", HumanizeDuration(fm.OldestItemAgeSeconds)))

	lines = append(lines, "")
	lines = append(lines, MetricLabelStyle.Render("Packet Loss:"))

	droppedStyle := GoodValueStyle
	if fm.MessagesDroppedTotal > 0 {
		droppedStyle = WarnValueStyle
	}
	if fm.DropRatePercent > 5 {
		droppedStyle = BadValueStyle
	}
	lines = append(lines, RenderColoredMetric("  Dropped", fmt.Sprintf("%d", fm.MessagesDroppedTotal), droppedStyle))

	evictStyle := GoodValueStyle
	if fm.ContextEvictionsTotal > 10 {
		evictStyle = WarnValueStyle
	}
	if fm.ContextEvictionsTotal > 50 {
		evictStyle = BadValueStyle
	}
	lines = append(lines, RenderColoredMetric("  Evicted", fmt.Sprintf("%d", fm.ContextEvictionsTotal), evictStyle))

	dropRateStyle := GoodValueStyle
	if fm.DropRatePercent > 1 {
		dropRateStyle = WarnValueStyle
	}
	if fm.DropRatePercent > 5 {
		dropRateStyle = BadValueStyle
	}
	lines = append(lines, RenderColoredMetric("  Drop Rate", fmt.Sprintf("%.1f%%", fm.DropRatePercent), dropRateStyle))

	return RenderPanel("LLM Context", strings.Join(lines, "\n"), width)
}

func renderPayloadPanel(fm metrics.FeedMetrics, width int) string {
	var lines []string

	lines = append(lines, RenderMetric("Last Payload", HumanizeBytesInt(fm.PayloadSizeLastBytes)))
	lines = append(lines, RenderMetric("Avg Payload", HumanizeBytesInt(int(fm.PayloadSizeAvgBytes))))
	lines = append(lines, RenderMetric("Max Payload", HumanizeBytesInt(fm.PayloadSizeMaxBytes)))

	return RenderPanel("Payload Size", strings.Join(lines, "\n"), width)
}

func renderLLMPanel(fm metrics.FeedMetrics, width int) string {
	var lines []string

	lines = append(lines, RenderMetric("Total Requests", fmt.Sprintf("%d", fm.LLMRequestsTotal)))

	lines = append(lines, "")
	lines = append(lines, MetricLabelStyle.Render("Last Request:"))
	lines = append(lines, RenderMetric("  Input Tokens", fmt.Sprintf("%d", fm.InputTokensLast)))
	lines = append(lines, RenderMetric("  Output Tokens", fmt.Sprintf("%d", fm.OutputTokensLast)))

	lines = append(lines, "")
	lines = append(lines, MetricLabelStyle.Render("Session Totals:"))
	lines = append(lines, RenderMetric("  Input Tokens", fmt.Sprintf("%d", fm.InputTokensTotal)))
	lines = append(lines, RenderMetric("  Output Tokens", fmt.Sprintf("%d", fm.OutputTokensTotal)))
	totalTokens := fm.InputTokensTotal + fm.OutputTokensTotal
	lines = append(lines, RenderMetric("  Total Tokens", fmt.Sprintf("%d", totalTokens)))

	lines = append(lines, "")
	lines = append(lines, RenderMetric("Events in Context", fmt.Sprintf("%d", fm.EventsInContextCurrent)))

	ctxStyle := ColorByThreshold(fm.ContextUtilizationPercent, 50, 80, false)
	ctxBar := RenderContextBar(fm.ContextUtilizationPercent, width-20)
	lines = append(lines, RenderColoredMetric("Context Usage", fmt.Sprintf("%.1f%%", fm.ContextUtilizationPercent), ctxStyle))
	lines = append(lines, ctxBar)

	lines = append(lines, "")
	lines = append(lines, MetricLabelStyle.Render("Timing:"))

	ttftStyle := GoodValueStyle
	if fm.TTFTMs > 1000 {
		ttftStyle = WarnValueStyle
	}
	if fm.TTFTMs > 3000 {
		ttftStyle = BadValueStyle
	}
	lines = append(lines, RenderColoredMetric("  TTFT (last)", fmt.Sprintf("%.0fms", fm.TTFTMs), ttftStyle))
	lines = append(lines, RenderMetric("  TTFT (avg)", fmt.Sprintf("%.0fms", fm.TTFTAvgMs)))

	genStyle := GoodValueStyle
	if fm.GenerationTimeMs > 5000 {
		genStyle = WarnValueStyle
	}
	if fm.GenerationTimeMs > 10000 {
		genStyle = BadValueStyle
	}
	lines = append(lines, RenderColoredMetric("  Gen Time (last)", fmt.Sprintf("%.0fms", fm.GenerationTimeMs), genStyle))
	lines = append(lines, RenderMetric("  Gen Time (avg)", fmt.Sprintf("%.0fms", fm.GenerationTimeAvgMs)))

	if len(fm.GenTimeHistory) > 0 {
		sparkWidth := width - 14
		if sparkWidth > 35 {
			sparkWidth = 35
		}
		sparkline := RenderSparkline(fm.GenTimeHistory, sparkWidth, true)
		lines = append(lines, MetricLabelStyle.Render("  Trend: ")+sparkline)
	}

	lines = append(lines, "")
	errStyle := GoodValueStyle
	if fm.LLMErrorsTotal > 0 {
		errStyle = BadValueStyle
	}
	lines = append(lines, RenderColoredMetric("Errors", fmt.Sprintf("%d", fm.LLMErrorsTotal), errStyle))

	return RenderPanel("LLM / Tokens", strings.Join(lines, "\n"), width)
}
