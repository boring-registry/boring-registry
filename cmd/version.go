package cmd

import (
	"fmt"

	"github.com/TierMobility/boring-registry/version"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Prints the version of the Boring Registry",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version.String())
	},
}
