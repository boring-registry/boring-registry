package cmd

import (
	"fmt"
	"os"
	"regexp"

	"github.com/hashicorp/go-version"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	flagRecursive                bool
	flagIgnoreExistingModule     bool
	flagVersionConstraintsRegex  string
	flagVersionConstraintsSemver string
)

var (
	versionConstraintsRegex  *regexp.Regexp
	versionConstraintsSemver version.Constraints
)

func init() {
	rootCmd.AddCommand(uploadCmd)
	uploadCmd.Flags().BoolVar(&flagRecursive, "recursive", true, "Recursively traverse <dir> and upload all modules in subdirectories")
	uploadCmd.Flags().BoolVar(&flagIgnoreExistingModule, "ignore-existing", true, "Ignore already existing modules. If set to false upload will fail immediately if a module already exists in that version")
	uploadCmd.Flags().StringVar(&flagVersionConstraintsRegex, "version-constraints-regex", "", "Limit the module versions that are eligible for upload with a regex that a version has to match.\n"+
		"Can be combined with the -version-constraints-semver flag")
	uploadCmd.Flags().StringVar(&flagVersionConstraintsSemver, "version-constraints-semver", "", "Limit the module versions that are eligible for upload with version constraints.\n"+
		"The version string has to be formatted as a string literal containing one or more conditions, which are separated by commas. Can be combined with the -version-constrained-regex flag")
}

var uploadCmd = &cobra.Command{
	Use:   "upload [flags] MODULE",
	Short: "Upload modules",
	RunE: func(cmd *cobra.Command, args []string) error {
		registry, err := setupRegistry()
		if err != nil {
			return errors.Wrap(err, "failed to setup registry")
		}

		if len(args) == 0 {
			return fmt.Errorf("missing argument")
		}

		if _, err := os.Stat(args[0]); errors.Is(err, os.ErrNotExist) {
			return err
		}

		// Validate the semver version constraints
		if flagVersionConstraintsSemver != "" {
			constraints, err := version.NewConstraint(flagVersionConstraintsSemver)
			if err != nil {
				return err
			}
			versionConstraintsSemver = constraints
		}

		// Validate the regex version constraints
		if flagVersionConstraintsRegex != "" {
			constraints, err := regexp.Compile(flagVersionConstraintsRegex)
			if err != nil {
				return fmt.Errorf("invalid regex given: %v", err)
			}
			versionConstraintsRegex = constraints
		}

		return archiveModules(args[0], registry)
	},
}
