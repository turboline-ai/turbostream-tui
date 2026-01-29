package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// LogoLines contains the ASCII art logo
var LogoLines = []string{
	"████████╗██╗   ██╗██████╗ ██████╗  ██████╗ ███████╗████████╗██████╗ ███████╗ █████╗ ███╗   ███╗",
	"╚══██╔══╝██║   ██║██╔══██╗██╔══██╗██╔═══██╗██╔════╝╚══██╔══╝██╔══██╗██╔════╝██╔══██╗████╗ ████║",
	"   ██║   ██║   ██║██████╔╝██████╔╝██║   ██║███████╗   ██║   ██████╔╝█████╗  ███████║██╔████╔██║",
	"   ██║   ██║   ██║██╔══██╗██╔══██╗██║   ██║╚════██║   ██║   ██╔══██╗██╔══╝  ██╔══██║██║╚██╔╝██║",
	"   ██║   ╚██████╔╝██║  ██║██████╔╝╚██████╔╝███████║   ██║   ██║  ██║███████╗██║  ██║██║ ╚═╝ ██║",
	"   ╚═╝    ╚═════╝ ╚═╝  ╚═╝╚═════╝  ╚═════╝ ╚══════╝   ╚═╝   ╚═╝  ╚═╝╚══════╝╚═╝  ╚═╝╚═╝     ╚═╝",
}

// RenderGradientLogo renders the ASCII logo with gradient colors
func RenderGradientLogo() string {
	var builder strings.Builder
	for i, line := range LogoLines {
		color := GradientColors[i%len(GradientColors)]
		style := lipgloss.NewStyle().Foreground(color).Bold(true)
		builder.WriteString(style.Render(line))
		builder.WriteString("\n")
	}
	return builder.String()
}

// RenderBoxWithTitle renders a box with the title embedded in the top border
func RenderBoxWithTitle(title, content string, width, height int, borderColor, titleColor lipgloss.Color) string {
	border := lipgloss.RoundedBorder()
	titleText := " " + title + " "

	// Calculate remaining dashes for top border
	remainingWidth := width - 3 - len(titleText)
	if remainingWidth < 0 {
		remainingWidth = 0
	}

	// Build the box
	var result strings.Builder

	// Top border with title
	result.WriteString(lipgloss.NewStyle().Foreground(borderColor).Render(border.TopLeft + border.Top))
	result.WriteString(lipgloss.NewStyle().Bold(true).Foreground(titleColor).Render(titleText))
	result.WriteString(lipgloss.NewStyle().Foreground(borderColor).Render(strings.Repeat(border.Top, remainingWidth) + border.TopRight))
	result.WriteString("\n")

	// Split content into lines
	contentLines := strings.Split(content, "\n")
	innerWidth := width - 4 // borders + padding

	// Calculate how many content lines we need
	contentHeight := height - 2 // subtract top and bottom borders
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Render content lines (pad or truncate to fit height)
	for i := 0; i < contentHeight; i++ {
		line := ""
		if i < len(contentLines) {
			line = contentLines[i]
		}

		// Get visual width of the line
		lineLen := lipgloss.Width(line)

		// Truncate if too long
		if lineLen > innerWidth {
			truncated := truncateWithANSI(line, innerWidth-3)
			line = truncated + "..."
			lineLen = lipgloss.Width(line)
		}

		// Pad to fill width
		if lineLen < innerWidth {
			line = line + strings.Repeat(" ", innerWidth-lineLen)
		}

		result.WriteString(lipgloss.NewStyle().Foreground(borderColor).Render(border.Left))
		result.WriteString(" " + line + " ")
		result.WriteString(lipgloss.NewStyle().Foreground(borderColor).Render(border.Right))
		result.WriteString("\n")
	}

	// Bottom border
	result.WriteString(lipgloss.NewStyle().Foreground(borderColor).Render(border.BottomLeft + strings.Repeat(border.Bottom, width-2) + border.BottomRight))

	return result.String()
}

// truncateWithANSI truncates a string while preserving ANSI escape sequences
func truncateWithANSI(line string, maxWidth int) string {
	truncated := ""
	currentWidth := 0
	inEscape := false

	for _, r := range line {
		if r == '\x1b' {
			inEscape = true
			truncated += string(r)
			continue
		}
		if inEscape {
			truncated += string(r)
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}
		if currentWidth >= maxWidth {
			break
		}
		truncated += string(r)
		currentWidth++
	}
	return truncated
}

// RenderPanel renders a titled panel with title embedded in the top border
func RenderPanel(title string, content string, width int) string {
	titleText := " " + title + " "
	border := lipgloss.RoundedBorder()

	remainingWidth := width - 3 - len(titleText)
	if remainingWidth < 0 {
		remainingWidth = 0
	}

	contentLines := strings.Split(content, "\n")
	var result strings.Builder

	// Add styled top border with title
	result.WriteString(lipgloss.NewStyle().Foreground(DarkCyanColor).Render(border.TopLeft + border.Top))
	result.WriteString(lipgloss.NewStyle().Bold(true).Foreground(BrightCyanColor).Render(titleText))
	result.WriteString(lipgloss.NewStyle().Foreground(DarkCyanColor).Render(strings.Repeat(border.Top, remainingWidth) + border.TopRight))
	result.WriteString("\n")

	// Add content lines with side borders
	innerWidth := width - 4
	for _, line := range contentLines {
		paddedLine := line
		lineLen := lipgloss.Width(line)
		if lineLen < innerWidth {
			paddedLine = line + strings.Repeat(" ", innerWidth-lineLen)
		}
		result.WriteString(lipgloss.NewStyle().Foreground(DarkCyanColor).Render(border.Left))
		result.WriteString(" " + paddedLine + " ")
		result.WriteString(lipgloss.NewStyle().Foreground(DarkCyanColor).Render(border.Right))
		result.WriteString("\n")
	}

	// Add bottom border
	result.WriteString(lipgloss.NewStyle().Foreground(DarkCyanColor).Render(border.BottomLeft + strings.Repeat(border.Bottom, width-2) + border.BottomRight))

	return result.String()
}

// RenderSparkline renders a sparkline chart from data values
func RenderSparkline(data []float64, width int, invertColor bool) string {
	if len(data) == 0 {
		return strings.Repeat("▁", width)
	}

	// Take most recent 'width' values
	start := 0
	if len(data) > width {
		start = len(data) - width
	}
	values := data[start:]

	// Find min/max for scaling
	minVal, maxVal := values[0], values[0]
	for _, v := range values {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}

	// Build sparkline
	var sb strings.Builder
	for _, v := range values {
		// Normalize to 0-7 (8 levels)
		level := 0
		if maxVal > minVal {
			level = int((v - minVal) / (maxVal - minVal) * 7)
		}
		if level > 7 {
			level = 7
		}
		if level < 0 {
			level = 0
		}

		char := SparklineChars[level]

		// Color based on level and invertColor setting
		var style lipgloss.Style
		if invertColor {
			// For latency: high = red (bad)
			switch {
			case level >= 6:
				style = SparklineRedStyle
			case level >= 4:
				style = SparklineYellowStyle
			default:
				style = SparklineGreenStyle
			}
		} else {
			// For throughput: high = green (good)
			switch {
			case level >= 6:
				style = SparklineGreenStyle
			case level >= 4:
				style = SparklineCyanStyle
			default:
				style = SparklineYellowStyle
			}
		}

		sb.WriteString(style.Render(char))
	}

	// Pad with empty bars if not enough data
	for i := len(values); i < width; i++ {
		sb.WriteString(lipgloss.NewStyle().Foreground(GrayColor).Render("▁"))
	}

	return sb.String()
}

// RenderMetric renders a single metric line
func RenderMetric(label string, value string) string {
	return MetricLabelStyle.Render(label+": ") + MetricValueStyle.Render(value)
}

// RenderColoredMetric renders a metric with conditional coloring
func RenderColoredMetric(label string, value string, style lipgloss.Style) string {
	return MetricLabelStyle.Render(label+": ") + style.Render(value)
}

// RenderContextBar renders a visual bar for context utilization
func RenderContextBar(percent float64, width int) string {
	if width < 10 {
		width = 10
	}

	filled := int(percent / 100 * float64(width))
	if filled > width {
		filled = width
	}

	var bar strings.Builder
	for i := 0; i < width; i++ {
		if i < filled {
			if percent > 80 {
				bar.WriteString(BadValueStyle.Render("█"))
			} else if percent > 50 {
				bar.WriteString(WarnValueStyle.Render("█"))
			} else {
				bar.WriteString(GoodValueStyle.Render("█"))
			}
		} else {
			bar.WriteString(lipgloss.NewStyle().Foreground(GrayColor).Render("░"))
		}
	}

	return "  [" + bar.String() + "]"
}

// HumanizeBytes converts bytes to human-readable format
func HumanizeBytes(bytes uint64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// HumanizeBytesInt converts int bytes to human-readable format
func HumanizeBytesInt(bytes int) string {
	if bytes < 0 {
		return "0 B"
	}
	return HumanizeBytes(uint64(bytes))
}

// HumanizeDuration converts seconds to human-readable duration
func HumanizeDuration(seconds float64) string {
	if seconds < 60 {
		return fmt.Sprintf("%.1fs", seconds)
	} else if seconds < 3600 {
		return fmt.Sprintf("%.1fm", seconds/60)
	} else if seconds < 86400 {
		return fmt.Sprintf("%.1fh", seconds/3600)
	}
	return fmt.Sprintf("%.1fd", seconds/86400)
}

// Truncate truncates a string to max length with ellipsis
func Truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

// WrapText wraps text to specified width
func WrapText(s string, width int) string {
	if width <= 0 {
		return s
	}
	var result strings.Builder
	words := strings.Fields(s)
	lineLen := 0
	for _, word := range words {
		wordLen := len(word)
		if lineLen+wordLen+1 > width && lineLen > 0 {
			result.WriteString("\n")
			lineLen = 0
		}
		if lineLen > 0 {
			result.WriteString(" ")
			lineLen++
		}
		result.WriteString(word)
		lineLen += wordLen
	}
	return result.String()
}

// FeedItemConnectedIcon returns the connected icon
func FeedItemConnectedIcon() string {
	return lipgloss.NewStyle().Foreground(GreenColor).Render("●")
}

// FeedItemDisconnectedIcon returns the disconnected icon
func FeedItemDisconnectedIcon() string {
	return lipgloss.NewStyle().Foreground(RedColor).Render("●")
}
