package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/boring-registry/boring-registry/pkg/module"
	"github.com/boring-registry/boring-registry/pkg/storage"

	"github.com/hashicorp/go-version"
	"github.com/spf13/cobra"
)

const (
	ModuleSpecFileName = "boring-registry.hcl"
)

var (
	flagRecursive                bool
	flagIgnoreExistingModule     bool
	flagVersionConstraintsRegex  string
	flagVersionConstraintsSemver string
	flagModuleVersion            string
)

var (
	moduleUploader  *ModuleUploadRunner
	uploadModuleCmd = &cobra.Command{
		Use:          "module MODULE",
		SilenceUsage: true,
		PreRunE:      moduleUploadPreRun,

		// The moduleUploader variable is initially nil, so we need this wrapper hack to make sure we reference it when it has been initialized
		RunE: func(cmd *cobra.Command, args []string) error {
			return moduleUploader.Run(cmd, args)
		},
	}
)

func init() {
	uploadModuleCmd.PersistentFlags().StringVar(&flagModuleVersion, "version", "", "Specify the version of the module to upload. Mutually exclusive with --recursive module discovery.")
}

type ModuleUploadConfig struct {
	Recursive                bool
	IgnoreExistingModule     bool
	VersionConstraintsRegex  *regexp.Regexp
	VersionConstraintsSemver version.Constraints
	ModuleVersion            *version.Version
}

func (m *ModuleUploadConfig) Validate() error {
	errs := []error{}
	if m.Recursive && m.ModuleVersion != nil {
		errs = append(errs, errors.New("providing a module version is not supported when traversing recursively, only one of the two options can be provided"))
	}

	return errors.Join(errs...)
}

func NewModuleUploadConfig(opts ...ModuleUploadConfigOption) *ModuleUploadConfig {
	// The following are the default configuration settings
	m := &ModuleUploadConfig{
		Recursive:                true,
		IgnoreExistingModule:     true,
		VersionConstraintsRegex:  nil,
		VersionConstraintsSemver: nil,
		ModuleVersion:            nil,
	}

	for _, option := range opts {
		option(m)
	}

	return m
}

func NewModuleUploadConfigFromFlags() (*ModuleUploadConfig, error) {
	errs := []error{}
	opts := []ModuleUploadConfigOption{
		WithModuleUploadConfigIgnoreExistingModule(flagIgnoreExistingModule),
		WithModuleUploadConfigRecursive(flagRecursive),
	}

	// Validate the semver version constraints
	if flagVersionConstraintsSemver != "" {
		constraints, err := version.NewConstraint(flagVersionConstraintsSemver)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to parse version-constraints-semver flag: %w", err))
		}
		opts = append(opts, WithModuleUploadConfigVersionConstraintsSemver(constraints))
	}

	// Validate the regex version constraints
	if flagVersionConstraintsRegex != "" {
		constraints, err := regexp.Compile(flagVersionConstraintsRegex)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to parse version-constraints-regex flag: %w", err))
		}
		opts = append(opts, WithModuleUploadConfigVersionConstraintsRegex(constraints))
	}

	if flagModuleVersion != "" {
		v, err := version.NewSemver(flagModuleVersion)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to parse version flag %s: %w", flagModuleVersion, err))
		}
		opts = append(opts, WithModuleUploadConfigModuleVersion(v))
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return NewModuleUploadConfig(opts...), nil
}

type ModuleUploadConfigOption func(*ModuleUploadConfig)

func WithModuleUploadConfigRecursive(recursive bool) ModuleUploadConfigOption {
	return func(m *ModuleUploadConfig) {
		m.Recursive = recursive
	}
}

func WithModuleUploadConfigIgnoreExistingModule(ignore bool) ModuleUploadConfigOption {
	return func(m *ModuleUploadConfig) {
		m.IgnoreExistingModule = ignore
	}
}

func WithModuleUploadConfigVersionConstraintsRegex(re *regexp.Regexp) ModuleUploadConfigOption {
	return func(m *ModuleUploadConfig) {
		m.VersionConstraintsRegex = re
	}
}

func WithModuleUploadConfigVersionConstraintsSemver(constraints version.Constraints) ModuleUploadConfigOption {
	return func(m *ModuleUploadConfig) {
		m.VersionConstraintsSemver = constraints
	}
}

func WithModuleUploadConfigModuleVersion(v *version.Version) ModuleUploadConfigOption {
	return func(m *ModuleUploadConfig) {
		m.ModuleVersion = v
	}
}

// The main idea of ModuleUploadRunner is to have a struct that can be mocked more easily in tests
type ModuleUploadRunner struct {
	storage module.Storage
	config  *ModuleUploadConfig

	// The following functions can be overridden for mocking
	Discover func(string) error
	Archive  func(string) (io.Reader, error)
	Process  func(string) error
}

func (m *ModuleUploadRunner) Run(cmd *cobra.Command, args []string) error {
	if err := m.config.Validate(); err != nil {
		return err
	}

	if len(args) == 0 {
		return fmt.Errorf("missing path argument to module directory")
	} else if len(args) > 1 {
		return fmt.Errorf("only a single module path argument is supported at a time")
	}

	if _, err := os.Stat(args[0]); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to locate module directory: %w", err)
	}

	return m.Discover(args[0])
}

func (m *ModuleUploadRunner) InitializeMethods() {
	m.Discover = m.walkModules
	m.Archive = archiveModule
	m.Process = m.processModule
}

func (m *ModuleUploadRunner) walkModules(root string) error {
	modulePaths := []string{} // holds paths to all discovered module spec files
	if m.config.Recursive {
		if err := filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return fmt.Errorf("walk-related error: %w", err)
			}

			if fi.Name() != ModuleSpecFileName {
				return nil
			}
			modulePaths = append(modulePaths, path)
			return nil
		}); err != nil {
			return err
		}
	} else {
		modulePaths = append(modulePaths, filepath.Join(root, ModuleSpecFileName))
	}

	for _, path := range modulePaths {
		if processErr := m.Process(path); processErr != nil {
			return fmt.Errorf("failed to process module at %s: %w", path, processErr)
		}
	}
	return nil
}

func (m *ModuleUploadRunner) processModule(path string) error {
	spec, err := module.ParseFile(path)
	if err != nil {
		return err
	}

	if m.config.ModuleVersion == nil {
		err = spec.ValidateWithVersion()
	} else {
		err = spec.ValidateWithoutVersion()
	}
	if err != nil {
		return fmt.Errorf("module specification at path %s failed validation: %w", path, err)
	}

	// The user can pass a flag that sets the version of the module.
	// In that case, recursive traversal/discovery is not allowed and the boring-registry.hcl file does not contain
	// the metadata.version attribute.
	if m.config.ModuleVersion != nil {
		spec.Metadata.Version = m.config.ModuleVersion.String()
	}

	slog.Debug("parsed module spec", slog.String("path", path), slog.String("name", spec.Name()))

	// Check if the module meets version constraints
	if m.config.VersionConstraintsSemver != nil {
		ok, err := spec.MeetsSemverConstraints(m.config.VersionConstraintsSemver)
		if err != nil {
			return err
		} else if !ok {
			// Skip the module, as it didn't pass the version constraints
			slog.Info("module doesn't meet semver version constraints, skipped", slog.String("name", spec.Name()), slog.String("version", spec.Metadata.Version))
			return nil
		}
	}

	if m.config.VersionConstraintsRegex != nil {
		ok := spec.MeetsRegexConstraints(m.config.VersionConstraintsRegex)
		if !ok {
			// Skip the module, as it didn't pass the regex version constraints
			slog.Info("module doesn't meet regex version constraints, skipped", slog.String("name", spec.Name()))
			return nil
		}
	}

	ctx := context.Background()
	providerAttrs := []any{
		slog.String("namespace", spec.Metadata.Namespace),
		slog.String("name", spec.Metadata.Name),
		slog.String("provider", spec.Metadata.Provider),
		slog.String("version", spec.Metadata.Version),
	}
	if _, err := m.storage.GetModule(ctx, spec.Metadata.Namespace, spec.Metadata.Name, spec.Metadata.Provider, spec.Metadata.Version); err != nil {
		if !errors.Is(err, module.ErrModuleNotFound) {
			slog.Error("failed to check if module exists in storage provider", append(providerAttrs, slog.Any("error", err))...)
			return fmt.Errorf("failed to check if module exists: %w", err)
		}
	} else {
		if m.config.IgnoreExistingModule {
			// We ignore the ErrModuleNotFound error, as it means the module version doesn't exist yet and we can proceed to upload it
			slog.Info("module already exists", providerAttrs...)
			return nil
		} else {
			slog.Error("module already exists", providerAttrs...)
			return fmt.Errorf("module version %s already exists", spec.Metadata.Version)
		}
	}

	moduleRoot := filepath.Dir(path)

	buf, err := m.Archive(moduleRoot)
	if err != nil {
		return err
	}

	if _, err := m.storage.UploadModule(ctx, spec.Metadata.Namespace, spec.Metadata.Name, spec.Metadata.Provider, spec.Metadata.Version, buf); err != nil {
		return fmt.Errorf("failed to upload module: %w", err)
	}

	slog.Info("module successfully uploaded", providerAttrs...)
	return nil
}

func NewModuleUploadRunnerWithDefaultConfig() *ModuleUploadRunner {
	return &ModuleUploadRunner{
		config: NewModuleUploadConfig(),
	}
}

func NewModuleUploadRunner(config *ModuleUploadConfig, s storage.Storage) *ModuleUploadRunner {
	return &ModuleUploadRunner{
		config:  config,
		storage: s,
	}
}

func moduleUploadPreRun(cmd *cobra.Command, args []string) error {
	storage, err := setupStorage(context.Background())
	if err != nil {
		return fmt.Errorf("failed to set up storage: %w", err)
	}

	cfg, err := NewModuleUploadConfigFromFlags()
	if err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}
	moduleUploader = NewModuleUploadRunner(cfg, storage)
	moduleUploader.InitializeMethods()
	return nil
}

func archiveModule(root string) (io.Reader, error) {
	buf := new(bytes.Buffer)
	// ensure the src actually exists before trying to tar it
	if _, err := os.Stat(root); err != nil {
		return buf, fmt.Errorf("unable to tar files: %w", err)
	}

	gw := gzip.NewWriter(buf)
	defer func() {
		if err := gw.Close(); err != nil {
			slog.Error("failed to close gzip writer", slog.String("module-root", root), slog.Any("error", err))
		}
	}()

	tw := tar.NewWriter(gw)
	defer func() {
		if err := tw.Close(); err != nil {
			slog.Error("failed to close tar writer", slog.String("module-root", root), slog.Any("error", err))
		}
	}()

	err := filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
		// return on any error
		if err != nil {
			return err
		}

		// return on non-regular files
		if !fi.Mode().IsRegular() {
			return nil
		}

		// create a new dir/file header
		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			return err
		}

		// update the name to correctly reflect the desired destination when untaring
		header.Name = archiveFileHeaderName(path, root)

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		data, err := os.Open(path)
		if err != nil {
			return err
		}

		if _, err := io.Copy(tw, data); err != nil {
			return err
		}

		// manually close here after each file operation; deferring would cause each file close
		// to wait until all operations have completed.
		if err := data.Close(); err != nil {
			return fmt.Errorf("failed to close file %s: %w", path, err)
		}

		return nil
	})

	return buf, err
}

func archiveFileHeaderName(path, root string) string {
	// Check if the module is uploaded non-recursively from the current directory
	if root == "." {
		return path
	}

	// Remove the root prefix from the path
	if strings.HasPrefix(path, root) {
		relativePath := strings.TrimPrefix(path, root)

		// the leading slash needs to be removed
		return strings.TrimPrefix(relativePath, "/")
	}

	return path
}
