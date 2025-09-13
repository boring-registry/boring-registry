package cmd

import (
	//"context"
	//"io"
	//"os"
	//"path/filepath"
	"testing"

	//"github.com/boring-registry/boring-registry/pkg/core"
	"github.com/boring-registry/boring-registry/pkg/module"

	"github.com/spf13/cobra"
)

//type Mock struct{}
//
//func (m *Mock) GetModule(ctx context.Context, namespace, name, provider, version string) (core.Module, error) {
//	return core.Module{}, nil
//}
//
//func (m *Mock) ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]core.Module, error) {
//	return nil, nil
//}
//
//func (m *Mock) UploadModule(ctx context.Context, namespace, name, provider, version string, body io.Reader) (core.Module, error) {
//	return core.Module{}, nil
//}

func TestModuleUploadRunner_Run(t *testing.T) {
	validPath := t.TempDir()
	m := &moduleUploadRunner{
		//storage: &Mock{},
		archive: func(_ string, _ module.Storage) error { return nil },
	}

	tests := []struct {
		name                     string
		args                     []string
		versionConstraintsSemver string
		versionConstraintsRegex  string
		wantErr                  bool
	}{
		{
			name:    "no args returns error",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "more than a single args returns error",
			args:    []string{t.TempDir(), t.TempDir()},
			wantErr: true,
		},
		{
			name:    "non-existent path returns error",
			args:    []string{"/non/existent/path"},
			wantErr: true,
		},
		{
			name:                     "invalid semver constraint returns error",
			args:                     []string{validPath},
			versionConstraintsSemver: "invalid-semver",
			wantErr:                  true,
		},
		{
			name:                     "valid semver constraint",
			args:                     []string{validPath},
			versionConstraintsSemver: ">1.0.0",
			wantErr:                  false,
		},
		{
			name:                     "multiple valid semver constraint",
			args:                     []string{validPath},
			versionConstraintsSemver: ">1.0.0,<3.0.0",
			wantErr:                  false,
		},
		{
			name:                    "invalid regex constraint returns error",
			args:                    []string{validPath},
			versionConstraintsRegex: "[invalid-regex",
			wantErr:                 true,
		},
		{
			name:                    "valid regex constraint",
			args:                    []string{validPath},
			versionConstraintsRegex: "1\\.0\\.\\d+",
			wantErr:                 false,
		},
		{
			name:    "valid path",
			args:    []string{validPath},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset global flags
			flagVersionConstraintsSemver = tt.versionConstraintsSemver
			flagVersionConstraintsRegex = tt.versionConstraintsRegex

			cmd := &cobra.Command{}
			err := m.run(cmd, tt.args)

			if (err != nil) != tt.wantErr {
				t.Errorf("run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

//func generateModuleDirectory(t *testing.T, specContent string) string {
//	dir := t.TempDir()
//	os.MkdirAll(dir, 0755)
//	mf, err := os.OpenFile(filepath.Join(dir, "main.tf"), os.O_RDWR|os.O_CREATE, 0644)
//	if err != nil {
//		panic(err)
//	}
//	defer mf.Close()
//	f, err := os.OpenFile(filepath.Join(dir, moduleSpecFileName), os.O_RDWR|os.O_CREATE, 0644)
//	if err != nil {
//		panic(err)
//	}
//	defer f.Close()
//
//	_, err = f.WriteString(specContent)
//	if err != nil {
//		panic(err)
//	}
//
//	return dir
//}
