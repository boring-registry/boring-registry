package core

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrObjectNotFound = errors.New("failed to locate object")
)

type ProviderError struct {
	Reason     string
	Provider   *Provider
	StatusCode int
}

func (p ProviderError) Error() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s: ", p.Reason))
	if p.Provider.Hostname != "" {
		sb.WriteString(fmt.Sprintf("hostname=%s, ", p.Provider.Hostname))
	}
	if p.Provider.Namespace != "" {
		sb.WriteString(fmt.Sprintf("namespace=%s, ", p.Provider.Namespace))
	}
	if p.Provider.Name != "" {
		sb.WriteString(fmt.Sprintf("name=%s, ", p.Provider.Name))
	}
	if p.Provider.Version != "" {
		sb.WriteString(fmt.Sprintf("version=%s, ", p.Provider.Version))
	}
	if p.Provider.OS != "" {
		sb.WriteString(fmt.Sprintf("os=%s, ", p.Provider.OS))
	}
	if p.Provider.Arch != "" {
		sb.WriteString(fmt.Sprintf("arch=%s, ", p.Provider.Arch))
	}
	message := sb.String()
	message = strings.TrimSuffix(message, ", ")
	return message
}
