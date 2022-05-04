package core

import "fmt"

// Module represents Terraform module metadata.
type Module struct {
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	Version     string `json:"version"`
	DownloadURL string `json:"download_url"`
}

// ID returns the module metadata in a compact format.
func (m *Module) ID(version bool) string {
	id := fmt.Sprintf("%s/%s/%s", m.Namespace, m.Name, m.Provider)
	if version {
		id = fmt.Sprintf("%s/%s", id, m.Version)
	}

	return id
}
