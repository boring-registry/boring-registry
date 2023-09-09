package core

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestProviderError_Error(t *testing.T) {
	type fields struct {
		Reason     string
		Provider   *Provider
		StatusCode int
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "with hostname, namespace, name",
			fields: fields{
				Reason: "this is the prefix",
				Provider: &Provider{
					Hostname:  "example.com",
					Namespace: "example",
					Name:      "test",
				},
			},
			want: "this is the prefix: hostname=example.com, namespace=example, name=test",
		},
		{
			name: "with hostname, namespace, name, os, and arch",
			fields: fields{
				Reason: "this is the prefix",
				Provider: &Provider{
					Hostname:  "example.com",
					Namespace: "example",
					Name:      "test",
					Version:   "0.1.2",
					OS:        "linux",
					Arch:      "amd64",
				},
			},
			want: "this is the prefix: hostname=example.com, namespace=example, name=test, version=0.1.2, os=linux, arch=amd64",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := ProviderError{
				Reason:     tt.fields.Reason,
				Provider:   tt.fields.Provider,
				StatusCode: tt.fields.StatusCode,
			}
			assert.Equalf(t, tt.want, p.Error(), "Error()")
		})
	}
}
