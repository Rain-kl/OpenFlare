// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package upload

import (
	"io"

	"github.com/Rain-kl/Wavelet/internal/model"
)

// OpenedUploadObject is the upload-domain view of a stored object stream.
type OpenedUploadObject struct {
	Body          io.ReadCloser
	ContentType   string
	ContentLength int64
	Upload        model.Upload
}

// Close closes the object body when present.
func (o OpenedUploadObject) Close() error {
	if o.Body == nil {
		return nil
	}
	return o.Body.Close()
}
