package gospinner

import (
	"github.com/fatih/color"
)

// ColorAttr is an adaptation of fatih color attributes as own type for easy usage for the lib users
type ColorAttr color.Attribute

// Color is and addaptation of fatih color as own type for easy usage for the lib users
type Color struct {
	*color.Color
}

const (
	FgBlack ColorAttr = iota + 30
	FgRed
	FgGreen
	FgYellow
	FgBlue
	FgMagenta
	FgCyan
	FgWhite
)

const (
	FgHiBlack ColorAttr = iota + 90
	FgHiRed
	FgHiGreen
	FgHiYellow
	FgHiBlue
	FgHiMagenta
	FgHiCyan
	FgHiWhite
)

// Handy funciton to create new color function
func newColor(cAttr ColorAttr) *Color {
	c := &Color{
		Color: color.New(color.Attribute(cAttr)),
	}
	return c
}

func noColor() {
	color.NoColor = true
}
