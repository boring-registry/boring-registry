package ui

import (
	"context"
	"testing"
)

func TestUI(t *testing.T) {
	// assert := assert.New(t)
	t.Parallel()

	ui := NewUI(context.Background(), WithColors(true))
	defer ui.Close()
	ui.Output("Hello", WithStyle(BoldStyle))
}
