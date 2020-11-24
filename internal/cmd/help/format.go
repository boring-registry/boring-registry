package help

import (
	"bytes"
	"flag"
	"fmt"
	"regexp"
	"strings"
	"text/tabwriter"

	"github.com/TierMobility/boring-registry/version"
	"github.com/mitchellh/go-glint"
	"github.com/peterbourgon/ff/v3/ffcli"
)

var (
	reHeader = regexp.MustCompile(`^(USAGE|EXAMPLE USAGE|FLAGS|SUBCOMMANDS|VERSION)$`)
)

func Format(v string) string {
	var buf bytes.Buffer
	d := glint.New()

	d.SetRenderer(&glint.TerminalRenderer{
		Output: &buf,
		Rows:   10,
		Cols:   180,
	})

	for _, line := range strings.Split(v, "\n") {
		if reHeader.MatchString(line) {
			d.Append(glint.Style(
				glint.Text(line),
				glint.Bold(),
			))

			continue
		}
		d.Append(glint.Text(line))
	}

	d.RenderFrame()
	return buf.String()
}

func UsageFunc(c *ffcli.Command) string {
	var b strings.Builder

	fmt.Fprintf(&b, Format("USAGE\n"))
	if c.ShortUsage != "" {
		fmt.Fprintf(&b, "  %s\n", c.ShortUsage)
	} else {
		fmt.Fprintf(&b, "  %s\n", c.Name)
	}
	fmt.Fprintf(&b, "\n")

	if c.LongHelp != "" {
		fmt.Fprintf(&b, "%s\n\n", c.LongHelp)
	}

	if len(c.Subcommands) > 0 {
		fmt.Fprintf(&b, Format("SUBCOMMANDS\n"))
		tw := tabwriter.NewWriter(&b, 0, 2, 2, ' ', 0)
		for _, subcommand := range c.Subcommands {
			fmt.Fprintf(tw, "  %s\t%s\n", subcommand.Name, subcommand.ShortHelp)
		}
		tw.Flush()
		fmt.Fprintf(&b, "\n")
	}

	if countFlags(c.FlagSet) > 0 {
		fmt.Fprintf(&b, Format("FLAGS\n"))
		tw := tabwriter.NewWriter(&b, 0, 2, 2, ' ', 0)
		c.FlagSet.VisitAll(func(f *flag.Flag) {
			def := f.DefValue
			if def == "" {
				def = "..."
			}
			fmt.Fprintf(tw, "  -%s %s\t%s\n", f.Name, def, f.Usage)
		})
		tw.Flush()
		fmt.Fprintf(&b, "\n")
	}

	fmt.Fprintf(&b, Format("VERSION\n"))
	fmt.Fprintf(&b, Format(fmt.Sprintf("boring-registry %s\n", version.VersionString())))

	return strings.TrimSpace(b.String())
}

func countFlags(fs *flag.FlagSet) (n int) {
	fs.VisitAll(func(*flag.Flag) { n++ })
	return n
}
