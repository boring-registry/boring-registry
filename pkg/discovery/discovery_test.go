package discovery

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoginV1Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		login       LoginV1
		wantErr     bool
		errContains string
	}{
		{
			name: "missing client-id",
			login: LoginV1{
				Client: "",
			},
			wantErr:     true,
			errContains: "client identifier value is required but not configured",
		},
		{
			name: "missing authz endpoint",
			login: LoginV1{
				Client: "boring-registry",
				Authz:  "",
			},
			wantErr:     true,
			errContains: "authz: is required but not configured",
		},
		{
			name: "missing token endpoint",
			login: LoginV1{
				Client: "boring-registry",
				Authz:  "/oauth2/authorization",
				Token:  "",
			},
			wantErr:     true,
			errContains: "token: is required but not configured",
		},
		{
			name: "only single port provided",
			login: LoginV1{
				Client: "boring-registry",
				Authz:  "/oauth2/authorization",
				Token:  "/oauth2/token",
				Ports:  []int{10020},
			},
			wantErr:     true,
			errContains: "ports: is expected to be a two-element array, but has 1 elements instead",
		},
		{
			name: "left port bound is larger than right",
			login: LoginV1{
				Client: "boring-registry",
				Authz:  "/oauth2/authorization",
				Token:  "/oauth2/token",
				Ports:  []int{10020, 10010},
			},
			wantErr:     true,
			errContains: "ports: the first array element is larger than the second",
		},
		{
			name: "left port bound is outside the allowed number of ports",
			login: LoginV1{
				Client: "boring-registry",
				Authz:  "/oauth2/authorization",
				Token:  "/oauth2/token",
				Ports:  []int{70000, 70001},
			},
			wantErr:     true,
			errContains: "ports: the first array element is outside the allowed port range of [0-65535]",
		},
		{
			name: "right port bound is outside the allowed number of ports",
			login: LoginV1{
				Client: "boring-registry",
				Authz:  "/oauth2/authorization",
				Token:  "/oauth2/token",
				Ports:  []int{10020, 65536},
			},
			wantErr:     true,
			errContains: "ports: the second array element is outside the allowed port range of [0-65535]",
		},
		{
			name: "scopes contain empty string",
			login: LoginV1{
				Client: "boring-registry",
				Authz:  "/oauth2/authorization",
				Token:  "/oauth2/token",
				Ports:  []int{10020, 10030},
				Scopes: []string{""},
			},
			wantErr:     true,
			errContains: "scopes: an array element is empty",
		},
		{
			name: "valid",
			login: LoginV1{
				Client: "boring-registry",
				Authz:  "/oauth2/authorization",
				Token:  "/oauth2/token",
				Ports:  []int{10020, 10030},
				Scopes: []string{"openid"},
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.login.Validate()
			if test.wantErr {
				assert.ErrorContains(t, err, test.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
