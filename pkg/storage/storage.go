package storage

import (
	"github.com/TierMobility/boring-registry/pkg/module"
	"github.com/TierMobility/boring-registry/pkg/provider"
)

const (
	DefaultModuleArchiveFormat = "tar.gz"
)

type Storage interface {
	provider.Storage
	module.Storage
}
