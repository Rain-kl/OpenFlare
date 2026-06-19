package logging

import edgelogging "github.com/Rain-kl/Wavelet/internal/apps/edge/logging"

func Setup() {
	edgelogging.Setup(edgelogging.Options{AddSource: true})
}