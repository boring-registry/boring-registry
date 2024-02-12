package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

var (
	flagDryRun bool
)

func init() {
	rootCmd.AddCommand(migrateCmd)
	migrateCmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Enable dry-run for the migration")
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

		if err := storageBackend.MigrateModules(ctx, flagDryRun); err != nil {
			return err
		}

		return storageBackend.MigrateProviders(ctx, flagDryRun)
	},
}
