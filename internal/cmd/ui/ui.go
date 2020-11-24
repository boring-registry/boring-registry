package ui

import (
	"context"
	"io"
	"os"

	"github.com/mitchellh/go-glint"
)

const (
	BoldStyle  = "bold"
	ErrorStyle = "error"
)

type UI struct {
	ctx context.Context
	gc  *glint.Document

	output        io.Writer
	colorsEnabled bool
}

func (ui *UI) Close() error {
	return ui.gc.Close()
}

func (ui *UI) Output(v string, options ...OutputOption) {
	config := &config{}

	for _, option := range options {
		option(config)
	}

	var styles []glint.StyleOption

	switch config.style {
	case BoldStyle:
		styles = append(styles, glint.Bold())
	}

	ui.gc.Append(
		glint.Style(
			glint.Text(v),
			styles...,
		),
	)
}

type config struct{ style string }

type OutputOption func(*config)

func WithStyle(style string) OutputOption {
	return func(cfg *config) {
		cfg.style = style
	}
}

type Option func(*UI)

func WithOutput(output io.Writer) Option {
	return func(ui *UI) {
		ui.output = output
	}
}
func WithColors(v bool) Option {
	return func(ui *UI) {
		ui.colorsEnabled = v
	}
}

func NewUI(ctx context.Context, options ...Option) *UI {
	ui := &UI{
		ctx:    ctx,
		output: os.Stdout,
		gc:     glint.New(),
	}

	for _, option := range options {
		option(ui)
	}

	ui.gc.SetRenderer(&glint.TerminalRenderer{
		Output: ui.output,
		Rows:   10,
		Cols:   180,
	})

	go ui.gc.Render(ctx)

	return ui
}
