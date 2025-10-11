package cmd

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(uploadCmd)

	uploadProviderCmd.Flags().StringVar(&flagFileSha256Sums, flagFileSha256SumsName, "", "The absolute path to the *_SHA256SUMS file")
	uploadProviderCmd.Flags().StringSliceVar(&flagProviderArchivePaths, "filenames-provider-archives", []string{}, "A list of file paths to provider ZIP archives")
	uploadProviderCmd.Flags().StringVar(&flagProviderNamespace, flagProviderNamespaceName, "", "The namespace under which the provider will be uploaded")
	for _, f := range []string{flagFileSha256SumsName, flagProviderNamespaceName} {
		if err := uploadProviderCmd.MarkFlagRequired(f); err != nil {
			panic(fmt.Errorf("failed to mark flag %s as required: %w", f, err))
		}
	}
	uploadCmd.AddCommand(uploadModuleCmd, uploadProviderCmd)

	uploadCmd.PersistentFlags().BoolVar(&flagRecursive, "recursive", true, "Recursively traverse <dir> and upload all modules in subdirectories")
	uploadCmd.PersistentFlags().BoolVar(&flagIgnoreExistingModule, "ignore-existing", true, "Ignore already existing modules. If set to false upload will fail immediately if a module already exists in that version")
	uploadCmd.PersistentFlags().StringVar(&flagVersionConstraintsRegex, "version-constraints-regex", "", `Limit the module versions that are eligible for upload with a regex that a version has to match.
Can be combined with the -version-constraints-semver flag`)
	uploadCmd.PersistentFlags().StringVar(&flagVersionConstraintsSemver, "version-constraints-semver", "", `Limit the module versions that are eligible for upload with version constraints.
The version string has to be formatted as a string literal containing one or more conditions, which are separated by commas.
Can be combined with the -version-constrained-regex flag`)
}

var (
	// uploadCmd uploads modules for legacy reasons.
	// It is recommended to use `upload module` instead.
	// This will eventually be deprecated and replaced.
	uploadCmd = &cobra.Command{
		Use:          "upload [flags] MODULE (WARNING: deprecated, use 'upload module' instead)",
		Short:        "Upload modules and providers",
		SilenceUsage: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			slog.Warn("using only the 'upload' command for modules is deprecated and will be removed in a future release. Please use 'upload module' instead.")
			return moduleUploader.preRun(cmd, args)
		},
		RunE: moduleUploader.run,
	}
)
