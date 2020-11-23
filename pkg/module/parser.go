package module

import (
	"errors"
	"io"
	"io/ioutil"
	"os"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/hcl"
)

// Spec represents a module spec with metadata.
type Spec struct {
	Metadata Metadata `hcl:"metadata" json:"metadata"`
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
	var result *multierror.Error

	if s.Metadata.Namespace == "" {
		result = multierror.Append(result, errors.New("metadata.namespace cannot be empty"))
	}

	if s.Metadata.Name == "" {
		result = multierror.Append(result, errors.New("metadata.name cannot be empty"))
	}

	if s.Metadata.Provider == "" {
		result = multierror.Append(result, errors.New("metadata.provider cannot be empty"))
	}

	if s.Metadata.Version == "" {
		result = multierror.Append(result, errors.New("metadata.version cannot be empty"))
	}

	if _, err := version.NewVersion(s.Metadata.Version); err != nil {
		result = multierror.Append(result, err)
	}

	return result.ErrorOrNil()
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

	buf, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	if err := hcl.Unmarshal(buf, spec); err != nil {
		return nil, err
	}

	if err := spec.Validate(); err != nil {
		return nil, err
	}

	return spec, nil
}
