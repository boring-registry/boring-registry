package module

import (
	"io"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		input         io.Reader
		expected      *Spec
		expectedError bool
	}{
		{
			name: "valid spec with version",
			input: strings.NewReader(`
            metadata {
              name      = "s3"
              namespace = "example"
              version   = "1.0.0"
              provider  = "aws"
            }
			`),
			expected: &Spec{
				Metadata{
					Name:      "s3",
					Namespace: "example",
					Version:   "1.0.0",
					Provider:  "aws",
				},
			},
			expectedError: false,
		},
		{
			name: "valid spec without version",
			input: strings.NewReader(`
            metadata {
              name      = "s3"
              namespace = "example"
              provider  = "aws"
            }
			`),
			expected: &Spec{
				Metadata{
					Name:      "s3",
					Namespace: "example",
					Provider:  "aws",
				},
			},
			expectedError: false,
		},
		{
			name: "valid spec with empty version",
			input: strings.NewReader(`
            metadata {
              name      = "s3"
              namespace = "example"
              provider  = "aws"
              version   = ""
            }
			`),
			expected: &Spec{
				Metadata{
					Name:      "s3",
					Namespace: "example",
					Version:   "", // default "null" value
					Provider:  "aws",
				},
			},
			expectedError: false,
		},
		{
			name: "spec with complex version",
			input: strings.NewReader(`
            metadata {
              name      = "s3"
              namespace = "example"
              version   = "1.0.0-beta+build.123"
              provider  = "aws"
            }
			`),
			expected: &Spec{
				Metadata{
					Name:      "s3",
					Namespace: "example",
					Version:   "1.0.0-beta+build.123",
					Provider:  "aws",
				},
			},
			expectedError: false,
		},
		{
			name:          "empty input",
			input:         strings.NewReader(``),
			expected:      nil,
			expectedError: true,
		},
		{
			name:          "invalid HCL syntax",
			input:         strings.NewReader(`foo: bar`),
			expected:      nil,
			expectedError: true,
		},
		{
			name: "invalid HCL with unclosed block",
			input: strings.NewReader(`
			metadata {
			  name = "s3"
			`),
			expected:      nil,
			expectedError: true,
		},
		{
			name:          "missing metadata block",
			input:         strings.NewReader(`other_block { foo = "bar" }`),
			expected:      nil,
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			spec, err := Parse(tc.input)

			if tc.expectedError {
				assert.Error(t, err)
				assert.Nil(t, spec)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, spec)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		spec          *Spec
		expectedError bool
		errorContains string
	}{
		{
			name: "valid spec with all required fields",
			spec: &Spec{
				Metadata{
					Name:      "s3",
					Namespace: "example",
					Provider:  "aws",
					Version:   "1.0.0",
				},
			},
			expectedError: false,
		},
		{
			name: "missing namespace",
			spec: &Spec{
				Metadata{
					Name:      "s3",
					Namespace: "",
					Provider:  "aws",
					Version:   "1.0.0",
				},
			},
			expectedError: true,
			errorContains: "metadata.namespace cannot be empty",
		},
		{
			name: "missing name",
			spec: &Spec{
				Metadata{
					Name:      "",
					Namespace: "example",
					Provider:  "aws",
					Version:   "1.0.0",
				},
			},
			expectedError: true,
			errorContains: "metadata.name cannot be empty",
		},
		{
			name: "missing provider",
			spec: &Spec{
				Metadata{
					Name:      "s3",
					Namespace: "example",
					Provider:  "",
					Version:   "1.0.0",
				},
			},
			expectedError: true,
			errorContains: "metadata.provider cannot be empty",
		},
		{
			name: "missing multiple fields",
			spec: &Spec{
				Metadata{
					Name:      "",
					Namespace: "",
					Provider:  "aws",
					Version:   "1.0.0",
				},
			},
			expectedError: true,
			errorContains: "metadata.namespace cannot be empty",
		},
		{
			name: "missing all required fields",
			spec: &Spec{
				Metadata{
					Name:      "",
					Namespace: "",
					Provider:  "",
					Version:   "1.0.0",
				},
			},
			expectedError: true,
			errorContains: "metadata.namespace cannot be empty",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Test using ValidateWithVersion since it calls the internal validate()
			err := tc.spec.ValidateWithVersion()

			if tc.expectedError {
				require.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateWithVersion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		spec          *Spec
		expectedError bool
		errorContains string
	}{
		{
			name: "valid semantic version",
			spec: &Spec{
				Metadata{
					Name:      "s3",
					Namespace: "example",
					Provider:  "aws",
					Version:   "1.0.0",
				},
			},
			expectedError: false,
		},
		{
			name: "valid prerelease version",
			spec: &Spec{
				Metadata{
					Name:      "s3",
					Namespace: "example",
					Provider:  "aws",
					Version:   "1.0.0-beta",
				},
			},
			expectedError: false,
		},
		{
			name: "valid version with build metadata",
			spec: &Spec{
				Metadata{
					Name:      "s3",
					Namespace: "example",
					Provider:  "aws",
					Version:   "1.0.0+build.123",
				},
			},
			expectedError: false,
		},
		{
			name: "valid version with prerelease and build metadata",
			spec: &Spec{
				Metadata{
					Name:      "s3",
					Namespace: "example",
					Provider:  "aws",
					Version:   "1.0.0-beta+build.123",
				},
			},
			expectedError: false,
		},
		{
			name: "version with v prefix is accepted",
			spec: &Spec{
				Metadata{
					Name:      "s3",
					Namespace: "example",
					Provider:  "aws",
					Version:   "v1.0.0",
				},
			},
			expectedError: false,
		},
		{
			name: "short version format is accepted",
			spec: &Spec{
				Metadata{
					Name:      "s3",
					Namespace: "example",
					Provider:  "aws",
					Version:   "1.0",
				},
			},
			expectedError: false,
		},
		{
			name: "missing version",
			spec: &Spec{
				Metadata{
					Name:      "s3",
					Namespace: "example",
					Provider:  "aws",
					Version:   "",
				},
			},
			expectedError: true,
			errorContains: "metadata.version cannot be empty",
		},
		{
			name: "invalid version format",
			spec: &Spec{
				Metadata{
					Name:      "s3",
					Namespace: "example",
					Provider:  "aws",
					Version:   "not-a-version",
				},
			},
			expectedError: true,
			errorContains: "failed to parse version",
		},
		{
			name: "version with leading whitespace",
			spec: &Spec{
				Metadata{
					Name:      "s3",
					Namespace: "example",
					Provider:  "aws",
					Version:   "  1.0.0",
				},
			},
			expectedError: true,
			errorContains: "failed to parse version",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.spec.ValidateWithVersion()

			if tc.expectedError {
				require.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateWithoutVersion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		spec          *Spec
		expectedError bool
		errorContains string
	}{
		{
			name: "valid spec without version",
			spec: &Spec{
				Metadata{
					Name:      "s3",
					Namespace: "example",
					Provider:  "aws",
					Version:   "",
				},
			},
			expectedError: false,
		},
		{
			name: "reject spec with version present",
			spec: &Spec{
				Metadata{
					Name:      "s3",
					Namespace: "example",
					Provider:  "aws",
					Version:   "1.0.0",
				},
			},
			expectedError: true,
			errorContains: "metadata.version must be empty",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.spec.ValidateWithoutVersion()

			if tc.expectedError {
				require.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSpec_Name(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name   string
		spec   *Spec
		expect string
	}{
		{
			name: "valid spec",
			spec: &Spec{
				Metadata{
					Name:      "s3",
					Namespace: "example",
					Provider:  "aws",
					Version:   "1.2.3",
				},
			},
			expect: "example/s3/aws/1.2.3",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.expect, tc.spec.Name())
		})
	}
}

func TestSpec_MeetsSemverConstraints(t *testing.T) {
	t.Parallel()
	constraintsHelper := func(constraints string) version.Constraints {
		c, err := version.NewConstraint(constraints)
		if err != nil {
			t.Fatalf("failed to construct constraint: %v", err)
		}
		return c
	}

	testCases := []struct {
		name        string
		spec        *Spec
		constraints version.Constraints
		expect      bool
		wantErr     bool
	}{
		{
			name: "valid spec which meets constraints",
			spec: &Spec{
				Metadata{
					Name:      "s3",
					Namespace: "example",
					Provider:  "aws",
					Version:   "1.2.3",
				},
			},
			constraints: constraintsHelper(">=1.0"),
			expect:      true,
		},
		{
			name: "invalid spec with version unset",
			spec: &Spec{
				Metadata{
					Name:      "s3",
					Namespace: "example",
					Provider:  "aws",
				},
			},
			constraints: constraintsHelper(">=1.0"),
			expect:      false,
			wantErr:     true,
		},
		{
			name: "valid spec with non-matching constraint",
			spec: &Spec{
				Metadata{
					Name:      "s3",
					Namespace: "example",
					Provider:  "aws",
					Version:   "1.2.3",
				},
			},
			constraints: constraintsHelper(">=2.0"),
			expect:      false,
			wantErr:     false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ok, err := tc.spec.MeetsSemverConstraints(tc.constraints)

			assert.Equal(t, tc.wantErr, err != nil)
			assert.Equal(t, tc.expect, ok)
		})
	}
}

func TestSpec_MeetsRegexConstraints(t *testing.T) {
	t.Parallel()
	regexHelper := func(regex string) *regexp.Regexp {
		re, err := regexp.Compile(regex)
		if err != nil {
			t.Fatalf("failed to construct constraint: %v", err)
		}
		return re
	}

	testCases := []struct {
		name        string
		spec        *Spec
		constraints *regexp.Regexp
		expect      bool
		wantErr     bool
	}{
		{
			name: "valid spec which meets constraints",
			spec: &Spec{
				Metadata{
					Name:      "s3",
					Namespace: "example",
					Provider:  "aws",
					Version:   "1.2.3",
				},
			},
			constraints: regexHelper("1\\.\\d+\\.\\d+"),
			expect:      true,
		},
		{
			name: "invalid spec with version unset",
			spec: &Spec{
				Metadata{
					Name:      "s3",
					Namespace: "example",
					Provider:  "aws",
				},
			},
			constraints: regexHelper("1\\.\\d+\\.\\d+"),
			expect:      false,
			wantErr:     true,
		},
		{
			name: "valid spec with non-matching constraint",
			spec: &Spec{
				Metadata{
					Name:      "s3",
					Namespace: "example",
					Provider:  "aws",
					Version:   "1.2.3",
				},
			},
			constraints: regexHelper("2\\.\\d+\\.\\d+"),
			expect:      false,
			wantErr:     false,
		},
		{
			name: "valid spec with regex matching pre-releases",
			spec: &Spec{
				Metadata{
					Name:      "s3",
					Namespace: "example",
					Provider:  "aws",
					Version:   "1.2.3-rc1",
				},
			},
			constraints: regexHelper("^[0-9]+\\.[0-9]+\\.[0-9]+-|\\d*[a-zA-Z-][0-9a-zA-Z-]*$"),
			expect:      true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ok, err := tc.spec.MeetsRegexConstraints(tc.constraints)

			assert.Equal(t, tc.wantErr, err != nil)
			assert.Equal(t, tc.expect, ok)
		})
	}
}

func TestParseFile(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name    string
		path    string
		expect  *Spec
		wantErr bool
	}{
		{
			name:    "non-existant path",
			path:    "/does/not/exist",
			wantErr: true,
		},
		{
			name:    "existing path",
			path:    "",
			wantErr: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var path string
			if tc.path == "" {
				path = t.TempDir()
			}
			result, err := ParseFile(path)

			assert.Equal(t, tc.wantErr, err != nil)
			assert.Equal(t, tc.expect, result)
		})
	}
}
