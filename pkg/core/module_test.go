package core

import (
	"testing"

	assertion "github.com/stretchr/testify/assert"
)

func TestModule_ID(t *testing.T) {
	t.Parallel()
	assert := assertion.New(t)

	testCases := []struct {
		name       string
		module     Module
		version    bool
		expectedID string
	}{
		{
			name: "valid provider without version disabled",
			module: Module{
				Namespace: "hashicorp",
				Name:      "random",
				Provider:  "aws",
				Version:   "v1.2.3",
			},
			version:    false,
			expectedID: "hashicorp/random/aws",
		},
		{
			name: "valid provider without version enabled",
			module: Module{
				Namespace: "hashicorp",
				Name:      "random",
				Provider:  "aws",
				Version:   "v1.2.3",
			},
			version:    true,
			expectedID: "hashicorp/random/aws/v1.2.3",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			id := tc.module.ID(tc.version)
			assert.Equal(tc.expectedID, id)
		})
	}
}
