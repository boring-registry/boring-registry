package cmd

import (
	"context"
	"fmt"
	"github.com/spf13/cobra"
)

var (
	//flagMigrateRecursive bool
	flagDryRun bool
	//flagTargetBucket string
)

func init() {
	rootCmd.AddCommand(migrateCmd)
	//rootCmd.Flags().BoolVar(&flagMigrateRecursive, "recursive", true, "Recursively traverse <dir> and migrate all modules in subdirectories")
	migrateCmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Enable dry-run for the migration")
	//rootCmd.Flags().StringVar(&flagTargetBucket, "target-bucket", "", "Optionally migrate modules to another bucket")
}

var migrateCmd = &cobra.Command{
	Use:   "migrate [flags] MODULE",
	Short: "Migrate modules",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		storageBackend, err := setupStorage(ctx)
		if err != nil {
			return fmt.Errorf("failed to setup storage: %w", err)
		}

		return storageBackend.MigrateModules(ctx, logger, flagDryRun)
	},
}
