package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"

	"github.com/boring-registry/boring-registry/pkg/module"

	"github.com/hashicorp/go-version"
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

var (
	moduleUploader  = &moduleUploadRunner{}
	uploadModuleCmd = &cobra.Command{
		Use:          "module MODULE",
		SilenceUsage: true,
		PreRunE:      moduleUploader.preRun,
		RunE:         moduleUploader.run,
	}
)

// The main idea of moduleUploadRunner is to have a struct that can be mocked more easily in tests
type moduleUploadRunner struct {
	storage module.Storage
	archive func(string, module.Storage) error
}

// preRun sets up the storage backend before running the upload command
func (m *moduleUploadRunner) preRun(cmd *cobra.Command, args []string) error {
	storage, err := setupStorage(context.Background())
	if err != nil {
		return fmt.Errorf("failed to set up storage: %w", err)
	}
	m.storage = storage
	m.archive = archiveModules
	return nil
}

func (m *moduleUploadRunner) run(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing path to module directory")
	} else if len(args) > 1 {
		return fmt.Errorf("only a single module is supported at a time")
	}

	if _, err := os.Stat(args[0]); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to locate module directory: %w", err)
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

	return m.archive(args[0], m.storage)
}
