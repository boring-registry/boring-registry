package gospinner

import "time"

// Symbols for the finishing actions
const (
	successSymbol = "✔"
	failureSymbol = "✖"
	warningSymbol = "⚠"
)

// AnimationKind represents the kind of the animation
type AnimationKind int

const (
	Ball AnimationKind = iota
	Column
	Slash
	Square
	Triangle
	Dots
	Dots2
	Pipe
	SimpleDots
	SimpleDotsScrolling
	GrowVertical
	GrowHorizontal
	Arrow
	BouncingBar
	BouncingBall
	Pong
	ProgressBar
)

// Animation represents an animation with frames and speed (recommended)
type Animation struct {
	interval time.Duration
	frames   []string
}

var animations = map[AnimationKind]Animation{
	Ball:                Animation{interval: 80 * time.Millisecond, frames: []string{"◐", "◓", "◑", "◒"}},
	Column:              Animation{interval: 80 * time.Millisecond, frames: []string{"☰", "☱", "☳", "☷", "☶", "☴"}},
	Slash:               Animation{interval: 130 * time.Millisecond, frames: []string{"-", "\\", "|", "/"}},
	Square:              Animation{interval: 110 * time.Millisecond, frames: []string{"▖", "▘", "▝", "▗"}},
	Triangle:            Animation{interval: 80 * time.Millisecond, frames: []string{"◢", "◣", "◤", "◥"}},
	Dots:                Animation{interval: 80 * time.Millisecond, frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}},
	Dots2:               Animation{interval: 80 * time.Millisecond, frames: []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}},
	Pipe:                Animation{interval: 100 * time.Millisecond, frames: []string{"┤", "┘", "┴", "└", "├", "┌", "┬", "┐"}},
	SimpleDots:          Animation{interval: 400 * time.Millisecond, frames: []string{".  ", ".. ", "...", "   "}},
	SimpleDotsScrolling: Animation{interval: 200 * time.Millisecond, frames: []string{".  ", ".. ", "...", " ..", "  .", "   "}},
	GrowVertical:        Animation{interval: 120 * time.Millisecond, frames: []string{"▁", "▃", "▄", "▅", "▆", "▇", "▆", "▅", "▄", "▃"}},
	GrowHorizontal:      Animation{interval: 120 * time.Millisecond, frames: []string{"▏", "▎", "▍", "▌", "▋", "▊", "▉", "▊", "▋", "▌", "▍", "▎"}},
	Arrow:               Animation{interval: 120 * time.Millisecond, frames: []string{"▹▹▹▹▹", "▸▹▹▹▹", "▹▸▹▹▹", "▹▹▸▹▹", "▹▹▹▸▹", "▹▹▹▹▸"}},
	BouncingBar:         Animation{interval: 80 * time.Millisecond, frames: []string{"[    ]", "[   =]", "[  ==]", "[ ===]", "[====]", "[=== ]", "[==  ]", "[=   ]"}},
	BouncingBall:        Animation{interval: 80 * time.Millisecond, frames: []string{"( ●    )", "(  ●   )", "(   ●  )", "(    ● )", "(     ●)", "(    ● )", "(   ●  )", "(  ●   )", "( ●    )", "(●     )"}},
	Pong:                Animation{interval: 80 * time.Millisecond, frames: []string{"▐⠂       ▌", "▐⠈       ▌", "▐ ⠂      ▌", "▐ ⠠      ▌", "▐  ⡀     ▌", "▐  ⠠     ▌", "▐   ⠂    ▌", "▐   ⠈    ▌", "▐    ⠂   ▌", "▐    ⠠   ▌", "▐     ⡀  ▌", "▐     ⠠  ▌", "▐      ⠂ ▌", "▐      ⠈ ▌", "▐       ⠂▌", "▐       ⠠▌", "▐       ⡀▌", "▐      ⠠ ▌", "▐      ⠂ ▌", "▐     ⠈  ▌", "▐     ⠂  ▌", "▐    ⠠   ▌", "▐    ⡀   ▌", "▐   ⠠    ▌", "▐   ⠂    ▌", "▐  ⠈     ▌", "▐  ⠂     ▌", "▐ ⠠      ▌", "▐ ⡀      ▌", "▐⠠       ▌"}},
	ProgressBar:         Animation{interval: 120 * time.Millisecond, frames: []string{"▒▒▒▒▒▒▒▒▒▒", "█▒▒▒▒▒▒▒▒▒", "███▒▒▒▒▒▒▒", "█████▒▒▒▒▒", "███████▒▒▒", "██████████"}},
}
