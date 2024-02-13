package module

import (
	"errors"
	"fmt"
	"io"
	"os"

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
	Version   string `hcl:"version" json:"version"`
}

// Validate ensures that a spec is valid.
func (s *Spec) Validate() error {
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

	if s.Metadata.Version == "" {
		errs = append(errs, errors.New("metadata.version cannot be empty"))
	}

	if _, err := version.NewVersion(s.Metadata.Version); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

func (s *Spec) Name() string {
	return fmt.Sprintf("%s/%s/%s/%s", s.Metadata.Namespace, s.Metadata.Name, s.Metadata.Provider, s.Metadata.Version)
}

// ParseFile parses a module spec file.
func ParseFile(path string) (*Spec, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

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

	if err := spec.Validate(); err != nil {
		return nil, err
	}

	return spec, nil
}
