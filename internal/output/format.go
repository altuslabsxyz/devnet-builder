package output

import (
	"strings"

	"github.com/fatih/color"
)

// Visual separator constants for error output formatting.
const (
	// SeparatorWidth is the width of separator lines.
	SeparatorWidth = 60

	// SeparatorChar is the character used for separator lines.
	SeparatorChar = "─"
)

// Separator returns a separator line of the default width.
func Separator() string {
	return strings.Repeat(SeparatorChar, SeparatorWidth)
}

// ColoredSeparator returns a colored separator line.
func ColoredSeparator(c *color.Color) string {
	return c.Sprint(Separator())
}

// RedSeparator returns a red separator line for errors.
func RedSeparator() string {
	return ColoredSeparator(color.New(color.FgRed))
}

// YellowSeparator returns a yellow separator line for warnings.
func YellowSeparator() string {
	return ColoredSeparator(color.New(color.FgYellow))
}

// CyanSeparator returns a cyan separator line for info.
func CyanSeparator() string {
	return ColoredSeparator(color.New(color.FgCyan))
}
