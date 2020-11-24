package help

import (
	"bytes"
	"strings"

	"github.com/mitchellh/go-glint"
)

func FormatHelp(v string) string {
	var buf bytes.Buffer
	d := glint.New()

	d.SetRenderer(&glint.TerminalRenderer{
		Output: &buf,
		Rows:   10,
		Cols:   180,
	})

	for _, line := range strings.Split(v, "\n") {
		d.Append(glint.Text(line))
	}

	d.RenderFrame()
	return buf.String()
}
