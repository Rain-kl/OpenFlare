package updater

import (
	edgeupdater "github.com/Rain-kl/Wavelet/internal/apps/edge/updater"
	"github.com/Rain-kl/Wavelet/internal/apps/flared/config"
)

type Service = edgeupdater.Service
type UpdateOptions = edgeupdater.UpdateOptions

func New() *Service {
	return edgeupdater.New(edgeupdater.Config{
		LocalVersion: config.Version,
		AssetPrefix:  "openflared",
		LogLabel:     "flared",
	})
}