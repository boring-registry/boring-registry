package storage

import (
	"strings"
	"testing"

	"github.com/boring-registry/boring-registry/pkg/core"
	"github.com/stretchr/testify/assert"
)

func TestProviderPath(t *testing.T) {
	t.Parallel()

	testCase := []struct {
		annotation        string
		prefix            string
		providerType      providerType
		hostname          string
		namespace         string
		name              string
		version           string
		os                string
		arch              string
		expectedPanic     bool
		expectedArchive   string
		expectedShasum    string
		expectedShasumSig string
	}{
		{
			annotation:    "every input is missing",
			expectedPanic: true,
		},
		{
			annotation:    "only prefix is passed",
			prefix:        "storage",
			providerType:  internalProviderType,
			expectedPanic: true,
		},
		{
			annotation:    "mirror type and hostname is missing",
			prefix:        "storage",
			providerType:  mirrorProviderType,
			hostname:      "",
			expectedPanic: true,
		},
		{
			annotation:    "mirror type and hostname is missing",
			prefix:        "storage",
			providerType:  mirrorProviderType,
			hostname:      "registry.terraform.io",
			namespace:     "hashicorp",
			name:          "",
			expectedPanic: true,
		},
		{
			annotation:        "all parameters for mirror storage are set",
			prefix:            "storagePrefix",
			providerType:      mirrorProviderType,
			hostname:          "registry.terraform.io",
			namespace:         "hashicorp",
			name:              "random",
			version:           "3.1.0",
			os:                "linux",
			arch:              "amd64",
			expectedPanic:     false,
			expectedArchive:   "storagePrefix/mirror/providers/registry.terraform.io/hashicorp/random/terraform-provider-random_3.1.0_linux_amd64.zip",
			expectedShasum:    "storagePrefix/mirror/providers/registry.terraform.io/hashicorp/random/terraform-provider-random_3.1.0_SHA256SUMS",
			expectedShasumSig: "storagePrefix/mirror/providers/registry.terraform.io/hashicorp/random/terraform-provider-random_3.1.0_SHA256SUMS.sig",
		},
		{
			annotation:        "all parameters for internal storage are set",
			prefix:            "storagePrefix",
			providerType:      internalProviderType,
			hostname:          "registry.terraform.io", // is set even though it should be omitted in the output
			namespace:         "hashicorp",
			name:              "random",
			version:           "3.1.0",
			os:                "linux",
			arch:              "amd64",
			expectedPanic:     false,
			expectedArchive:   "storagePrefix/providers/hashicorp/random/terraform-provider-random_3.1.0_linux_amd64.zip",
			expectedShasum:    "storagePrefix/providers/hashicorp/random/terraform-provider-random_3.1.0_SHA256SUMS",
			expectedShasumSig: "storagePrefix/providers/hashicorp/random/terraform-provider-random_3.1.0_SHA256SUMS.sig",
		},
	}

	for _, tc := range testCase {
		tc := tc
		t.Run(tc.annotation, func(t *testing.T) {
			defer func() {
				if tc.expectedPanic {
					if r := recover(); r == nil {
						t.Errorf("The code did not panic")
					}
				}
			}()

			archive, shasum, shasumSig := providerPath(tc.prefix, tc.providerType, tc.hostname, tc.namespace, tc.name, tc.version, tc.os, tc.arch)
			assert.Equal(t, tc.expectedArchive, archive)
			assert.Equal(t, tc.expectedShasum, shasum)
			assert.Equal(t, tc.expectedShasumSig, shasumSig)
		})
	}
}

func TestSigningKeysPath(t *testing.T) {
	t.Parallel()

	testCase := []struct {
		annotation string
		prefix     string
		pt         providerType
		hostname   string
		namespace  string
		expected   string
	}{
		{
			annotation: "internal keys with prefix and namespace",
			prefix:     "prefix",
			pt:         internalProviderType,
			namespace:  "hashicorp",
			expected:   "prefix/providers/hashicorp/signing-keys.json",
		},
		{
			annotation: "mirrored keys with prefix and namespace",
			prefix:     "prefix",
			pt:         mirrorProviderType,
			hostname:   "terraform.example.com",
			namespace:  "hashicorp",
			expected:   "prefix/mirror/providers/terraform.example.com/hashicorp/signing-keys.json",
		},
	}

	for _, tc := range testCase {
		tc := tc
		t.Run(tc.annotation, func(t *testing.T) {
			result := signingKeysPath(tc.prefix, tc.pt, tc.hostname, tc.namespace)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestReadSHASums(t *testing.T) {
	t.Parallel()

	testCase := []struct {
		annotation     string
		file           string
		name           string
		expectedError  bool
		expectedSHASum string
	}{
		{
			annotation: "name is empty",
			file: `d9ab41d556a48bd7059f0810cf020500635bfc696c9fc3adab5ea8915c1d886b  terraform-provider-random_3.1.0_darwin_amd64.zip
a3a9251fb15f93e4cfc1789800fc2d7414bbc18944ad4c5c98f466e6477c42bc  terraform-provider-random_3.1.0_darwin_arm64.zip
4f251b0eda5bb5e3dc26ea4400dba200018213654b69b4a5f96abee815b4f5ff  terraform-provider-random_3.1.0_freebsd_386.zip
738ed82858317ccc246691c8b85995bc125ac3b4143043219bd0437adc56c992  terraform-provider-random_3.1.0_freebsd_amd64.zip
3cd456047805bf639fbf2c761b1848880ea703a054f76db51852008b11008626  terraform-provider-random_3.1.0_freebsd_arm.zip
2bbb3339f0643b5daa07480ef4397bd23a79963cc364cdfbb4e86354cb7725bc  terraform-provider-random_3.1.0_linux_386.zip
d9e13427a7d011dbd654e591b0337e6074eef8c3b9bb11b2e39eaaf257044fd7  terraform-provider-random_3.1.0_linux_amd64.zip
7dbe52fac7bb21227acd7529b487511c91f4107db9cc4414f50d04ffc3cab427  terraform-provider-random_3.1.0_linux_arm64.zip
a543ec1a3a8c20635cf374110bd2f87c07374cf2c50617eee2c669b3ceeeaa9f  terraform-provider-random_3.1.0_linux_arm.zip
f7605bd1437752114baf601bdf6931debe6dc6bfe3006eb7e9bb9080931dca8a  terraform-provider-random_3.1.0_windows_386.zip
7011332745ea061e517fe1319bd6c75054a314155cb2c1199a5b01fe1889a7e2  terraform-provider-random_3.1.0_windows_amd64.zip`,
			name:          "",
			expectedError: true,
		},
		{
			annotation: "name is not in file",
			file: `d9ab41d556a48bd7059f0810cf020500635bfc696c9fc3adab5ea8915c1d886b  terraform-provider-random_3.1.0_darwin_amd64.zip
a3a9251fb15f93e4cfc1789800fc2d7414bbc18944ad4c5c98f466e6477c42bc  terraform-provider-random_3.1.0_darwin_arm64.zip
4f251b0eda5bb5e3dc26ea4400dba200018213654b69b4a5f96abee815b4f5ff  terraform-provider-random_3.1.0_freebsd_386.zip
738ed82858317ccc246691c8b85995bc125ac3b4143043219bd0437adc56c992  terraform-provider-random_3.1.0_freebsd_amd64.zip
3cd456047805bf639fbf2c761b1848880ea703a054f76db51852008b11008626  terraform-provider-random_3.1.0_freebsd_arm.zip
2bbb3339f0643b5daa07480ef4397bd23a79963cc364cdfbb4e86354cb7725bc  terraform-provider-random_3.1.0_linux_386.zip
d9e13427a7d011dbd654e591b0337e6074eef8c3b9bb11b2e39eaaf257044fd7  terraform-provider-random_3.1.0_linux_amd64.zip
7dbe52fac7bb21227acd7529b487511c91f4107db9cc4414f50d04ffc3cab427  terraform-provider-random_3.1.0_linux_arm64.zip
a543ec1a3a8c20635cf374110bd2f87c07374cf2c50617eee2c669b3ceeeaa9f  terraform-provider-random_3.1.0_linux_arm.zip
f7605bd1437752114baf601bdf6931debe6dc6bfe3006eb7e9bb9080931dca8a  terraform-provider-random_3.1.0_windows_386.zip
7011332745ea061e517fe1319bd6c75054a314155cb2c1199a5b01fe1889a7e2  terraform-provider-random_3.1.0_windows_amd64.zip`,
			name:          "terraform-provider-random_3.99.0_windows_386.zip",
			expectedError: true,
		},
		{
			annotation: "name is in file",
			file: `d9ab41d556a48bd7059f0810cf020500635bfc696c9fc3adab5ea8915c1d886b  terraform-provider-random_3.1.0_darwin_amd64.zip
a3a9251fb15f93e4cfc1789800fc2d7414bbc18944ad4c5c98f466e6477c42bc  terraform-provider-random_3.1.0_darwin_arm64.zip
4f251b0eda5bb5e3dc26ea4400dba200018213654b69b4a5f96abee815b4f5ff  terraform-provider-random_3.1.0_freebsd_386.zip
738ed82858317ccc246691c8b85995bc125ac3b4143043219bd0437adc56c992  terraform-provider-random_3.1.0_freebsd_amd64.zip
3cd456047805bf639fbf2c761b1848880ea703a054f76db51852008b11008626  terraform-provider-random_3.1.0_freebsd_arm.zip
2bbb3339f0643b5daa07480ef4397bd23a79963cc364cdfbb4e86354cb7725bc  terraform-provider-random_3.1.0_linux_386.zip
d9e13427a7d011dbd654e591b0337e6074eef8c3b9bb11b2e39eaaf257044fd7  terraform-provider-random_3.1.0_linux_amd64.zip
7dbe52fac7bb21227acd7529b487511c91f4107db9cc4414f50d04ffc3cab427  terraform-provider-random_3.1.0_linux_arm64.zip
a543ec1a3a8c20635cf374110bd2f87c07374cf2c50617eee2c669b3ceeeaa9f  terraform-provider-random_3.1.0_linux_arm.zip
f7605bd1437752114baf601bdf6931debe6dc6bfe3006eb7e9bb9080931dca8a  terraform-provider-random_3.1.0_windows_386.zip
7011332745ea061e517fe1319bd6c75054a314155cb2c1199a5b01fe1889a7e2  terraform-provider-random_3.1.0_windows_amd64.zip`,
			name:           "terraform-provider-random_3.1.0_linux_amd64.zip",
			expectedError:  false,
			expectedSHASum: "d9e13427a7d011dbd654e591b0337e6074eef8c3b9bb11b2e39eaaf257044fd7",
		},
	}

	for _, tc := range testCase {
		tc := tc
		t.Run(tc.annotation, func(t *testing.T) {
			result, err := readSHASums(strings.NewReader(tc.file), tc.name)
			if tc.expectedError {
				assert.Error(t, err)
				return
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tc.expectedSHASum, result)

		})
	}
}

func TestModuleFromObject(t *testing.T) {
	t.Parallel()

	testCase := []struct {
		annotation    string
		key           string
		fileExtension string
		expectedError bool
		result        core.Module
	}{
		{
			annotation:    "empty path",
			key:           "",
			expectedError: true,
		},
		{
			annotation:    "empty file extension",
			key:           "/",
			fileExtension: "",
			expectedError: true,
		},
		{
			annotation:    "valid key without prefix",
			key:           "/modules/hashicorp/consul/aws/hashicorp-consul-aws-0.11.0.tar.gz",
			fileExtension: "tar.gz",
			expectedError: false,
			result: core.Module{
				Namespace: "hashicorp",
				Name:      "consul",
				Provider:  "aws",
				Version:   "0.11.0",
			},
		},
		{
			annotation:    "valid key with prefix",
			key:           "/boring-registry/modules/hashicorp/consul/aws/hashicorp-consul-aws-0.11.0.tar.gz",
			fileExtension: "tar.gz",
			expectedError: false,
			result: core.Module{
				Namespace: "hashicorp",
				Name:      "consul",
				Provider:  "aws",
				Version:   "0.11.0",
			},
		},
		{
			annotation:    "valid key with longer prefix",
			key:           "/boring-registry/test/modules/hashicorp/consul/aws/hashicorp-consul-aws-0.11.0.tar.gz",
			fileExtension: "tar.gz",
			expectedError: false,
			result: core.Module{
				Namespace: "hashicorp",
				Name:      "consul",
				Provider:  "aws",
				Version:   "0.11.0",
			},
		},
		{
			annotation:    "key with another file extension than provided",
			key:           "/boring-registry/test/modules/hashicorp/consul/aws/hashicorp-consul-aws-0.11.0.zip",
			fileExtension: "tar.gz",
			expectedError: true,
		},
		{
			annotation:    "key with 4 hyphens in the file",
			key:           "/boring-registry/test/modules/hashicorp/consul/aws/hashicorp-consul-hashicorp-aws-0.11.0-beta1.tar.gz",
			fileExtension: "tar.gz",
			expectedError: true,
		},
		{
			annotation:    "module with a hyphen in the name",
			key:           "/boring-registry/test/modules/hashicorp/private-key/aws/hashicorp-private-key-aws-0.11.0.tar.gz",
			fileExtension: "tar.gz",
			expectedError: false,
			result: core.Module{
				Namespace: "hashicorp",
				Name:      "private-key",
				Provider:  "aws",
				Version:   "0.11.0",
			},
		},
		{
			annotation:    "key with pre-release version",
			key:           "/boring-registry/test/modules/hashicorp/consul/aws/hashicorp-consul-aws-0.11.0-beta1.tar.gz",
			fileExtension: "tar.gz",
			expectedError: false,
			result: core.Module{
				Namespace: "hashicorp",
				Name:      "consul",
				Provider:  "aws",
				Version:   "0.11.0-beta1",
			},
		},
	}

	for _, tc := range testCase {
		tc := tc
		t.Run(tc.annotation, func(t *testing.T) {
			result, err := moduleFromObject(tc.key, tc.fileExtension)
			if tc.expectedError {
				assert.Error(t, err)
				return
			} else {
				assert.NoError(t, err)
			}

			assert.EqualValues(t, tc.result, *result)
		})
	}
}
