// Package ui provides styling and UI components for the TurboStream TUI.
package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// Color palette - Cyan theme with Magenta accents
var (
	// Primary colors
	CyanColor       = lipgloss.Color("#00FFFF")
	DarkCyanColor   = lipgloss.Color("#008B8B")
	BrightCyanColor = lipgloss.Color("#00FFFF")
	DimCyanColor    = lipgloss.Color("#5F9EA0")

	// Neutral colors
	WhiteColor    = lipgloss.Color("#FFFFFF")
	GrayColor     = lipgloss.Color("#808080")
	DarkGrayColor = lipgloss.Color("#2D2D2D")

	// Status colors
	GreenColor = lipgloss.Color("#00FF00")
	RedColor   = lipgloss.Color("#FF6B6B")

	// Accent colors (Magenta for tabs/AI)
	MagentaColor     = lipgloss.Color("#FF00FF")
	DarkMagentaColor = lipgloss.Color("#8B008B")
	DimMagentaColor  = lipgloss.Color("#BA55D3")
)

// Tab styles
var (
	ActiveTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#000000")).
			Background(MagentaColor).
			Padding(0, 2).
			MarginRight(1)

	InactiveTabStyle = lipgloss.NewStyle().
				Foreground(DimMagentaColor).
				Background(DarkGrayColor).
				Padding(0, 2).
				MarginRight(1)

	TabBarStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(DarkMagentaColor).
			MarginBottom(1)
)

// Content styles
var (
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(DarkCyanColor).
			Padding(1, 2)

	HelpStyle = lipgloss.NewStyle().
			Foreground(DimCyanColor)

	ContentStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(DarkCyanColor).
			Padding(1, 2).
			Width(100)
)

// Dashboard panel styles
var (
	SummaryBarStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(WhiteColor).
			Background(lipgloss.Color("#1a1a2e")).
			Padding(0, 2).
			MarginBottom(1)

	MetricLabelStyle = lipgloss.NewStyle().
				Foreground(DimCyanColor)

	MetricValueStyle = lipgloss.NewStyle().
				Foreground(WhiteColor).
				Bold(true)

	GoodValueStyle = lipgloss.NewStyle().
			Foreground(GreenColor).
			Bold(true)

	WarnValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD700")).
			Bold(true)

	BadValueStyle = lipgloss.NewStyle().
			Foreground(RedColor).
			Bold(true)
)

// Feed list styles
var (
	FeedItemSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#000000")).
				Background(CyanColor).
				Bold(true).
				Padding(0, 1)

	FeedItemNormalStyle = lipgloss.NewStyle().
				Foreground(DimCyanColor).
				Padding(0, 1)
)

// SparklineChars for visualizing data trends
var SparklineChars = []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

// Sparkline color styles
var (
	SparklineGreenStyle  = lipgloss.NewStyle().Foreground(GreenColor)
	SparklineCyanStyle   = lipgloss.NewStyle().Foreground(CyanColor)
	SparklineYellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#F1C40F"))
	SparklineRedStyle    = lipgloss.NewStyle().Foreground(RedColor)
)

// Gradient colors for logo
var GradientColors = []lipgloss.Color{
	lipgloss.Color("#00FFFF"), // Cyan
	lipgloss.Color("#33CCFF"),
	lipgloss.Color("#6699FF"),
	lipgloss.Color("#9966FF"),
	lipgloss.Color("#CC33FF"),
	lipgloss.Color("#FF00FF"), // Magenta
}

// ColorByThreshold returns appropriate style based on thresholds
func ColorByThreshold(value, warnThreshold, badThreshold float64, inverted bool) lipgloss.Style {
	if inverted {
		if value >= badThreshold {
			return GoodValueStyle
		} else if value >= warnThreshold {
			return WarnValueStyle
		}
		return BadValueStyle
	}

	if value >= badThreshold {
		return BadValueStyle
	} else if value >= warnThreshold {
		return WarnValueStyle
	}
	return GoodValueStyle
}
