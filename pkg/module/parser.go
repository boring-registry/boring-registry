package module

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/hcl/v2/hclsimple"
)

// Spec represents a module spec with metadata.
type Spec struct {
	Metadata Metadata `hcl:"metadata,block" json:"metadata"`
}

// Metadata provides information about a given module version.
type Metadata struct {
	Namespace string `hcl:"namespace" json:"namespace"`
	Name      string `hcl:"name" json:"name"`
	Provider  string `hcl:"provider" json:"provider"`
	Version   string `hcl:"version,optional" json:"version,omitempty"`
}

// ValidateWithVersion ensures that a spec that should contain a version is valid.
func (s *Spec) ValidateWithVersion() error {
	var errs []error

	if s.Metadata.Version == "" {
		errs = append(errs, errors.New("metadata.version cannot be empty"))
	}

	if _, err := version.NewVersion(s.Metadata.Version); err != nil {
		errs = append(errs, fmt.Errorf("failed to parse version: %w", err))
	}

	return errors.Join(append(s.validate(), errs...)...)
}

func (s *Spec) ValidateWithoutVersion() error {
	var errs []error

	if s.Metadata.Version != "" {
		errs = append(errs, errors.New("metadata.version must be empty"))
	}

	return errors.Join(append(s.validate(), errs...)...)
}

func (s *Spec) validate() []error {
	var errs []error

	if s.Metadata.Namespace == "" {
		errs = append(errs, errors.New("metadata.namespace cannot be empty"))
	}

	if s.Metadata.Name == "" {
		errs = append(errs, errors.New("metadata.name cannot be empty"))
	}

	if s.Metadata.Provider == "" {
		errs = append(errs, errors.New("metadata.provider cannot be empty"))
	}

	return errs
}

func (s *Spec) Name() string {
	return fmt.Sprintf("%s/%s/%s/%s", s.Metadata.Namespace, s.Metadata.Name, s.Metadata.Provider, s.Metadata.Version)
}

// MeetsSemverConstraints checks whether a module version matches the given semver version constraints.
// Returns an unrecoverable error if there's an internal error.
// Otherwise, it returns a boolean indicating if the module meets the constraints
func (s *Spec) MeetsSemverConstraints(constraints version.Constraints) (bool, error) {
	v, err := version.NewSemver(s.Metadata.Version)
	if err != nil {
		return false, err
	}

	return constraints.Check(v), nil
}

// MeetsRegexConstraints checks whether a module version matches the regex.
// Returns a boolean indicating if the module meets the constraints
func (s *Spec) MeetsRegexConstraints(re *regexp.Regexp) bool {
	return re.MatchString(s.Metadata.Version)
}

// ParseFile parses a module spec file.
func ParseFile(path string) (*Spec, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := file.Close(); err != nil {
			slog.Warn("failed to close file", "path", path, "error", err)
		}
	}()

	return Parse(file)
}

// Parse parses a module spec.
func Parse(r io.Reader) (*Spec, error) {
	spec := &Spec{}

	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	if err := hclsimple.Decode("boring-registry.hcl", b, nil, spec); err != nil {
		return nil, err
	}

	return spec, nil
}
