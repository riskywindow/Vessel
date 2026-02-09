// Package styles provides centralized terminal styling for the Vessel CLI.
package styles

import (
	"os"

	"github.com/charmbracelet/lipgloss"
)

// ColorEnabled returns true if color output should be used.
func ColorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	return true
}

// Status colors.
var (
	Success = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))  // Green
	Error   = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // Red
	Warning = lipgloss.NewStyle().Foreground(lipgloss.Color("214")) // Orange/Yellow
	Info    = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))  // Blue
	Muted   = lipgloss.NewStyle().Foreground(lipgloss.Color("245")) // Gray
)

// Text styles.
var (
	Bold = lipgloss.NewStyle().Bold(true)
)

// Semantic styles.
var (
	AppName     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	ContainerID = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	Version     = lipgloss.NewStyle().Foreground(lipgloss.Color("213"))
)

// Header is used for section titles.
var Header = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))

// TableHeader is used for table column names.
var TableHeader = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("245"))

// Icons for check results.
func CheckPassIcon() string { return Success.Render("✓") }
func CheckFailIcon() string { return Error.Render("✗") }
func CheckWarnIcon() string { return Warning.Render("!") }

// Status icons for inline use.
func SuccessIcon() string { return Success.Render("✓") }
func ErrorIcon() string   { return Error.Render("✗") }
func WarningIcon() string { return Warning.Render("!") }
func InfoIcon() string    { return Info.Render("ℹ") }

// StatusDot returns a colored dot for container/app status.
func StatusDot(status string) string {
	switch status {
	case "running":
		return Success.Render("●")
	case "stopped":
		return Muted.Render("○")
	case "failed":
		return Error.Render("✗")
	case "deploying":
		return Warning.Render("◉")
	case "healthy":
		return Success.Render("✓")
	case "unhealthy":
		return Error.Render("✗")
	default:
		return Muted.Render("?")
	}
}

// Render applies a style to text, respecting NO_COLOR.
func Render(s lipgloss.Style, text string) string {
	if !ColorEnabled() {
		return text
	}
	return s.Render(text)
}
