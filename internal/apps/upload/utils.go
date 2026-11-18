// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package upload

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif"  // Register GIF decoder for image.Decode
	_ "image/jpeg" // Register JPEG decoder for image.Decode
	_ "image/png"  // Register PNG decoder for image.Decode
	"io"
	"strings"

	"github.com/deepteams/webp"
	_ "golang.org/x/image/webp" // Register WebP decoder for image.Decode
)

// ValidateS3Key validates an S3 object key for safety.
func ValidateS3Key(key string) error {
	if key == "" {
		return errors.New(ErrS3KeyRequired)
	}

	if len(key) > MaxS3KeyLength {
		return fmt.Errorf(ErrS3KeyTooLongFormat, MaxS3KeyLength)
	}

	if strings.HasPrefix(key, "/") {
		return errors.New(ErrS3KeyStartsWithSlash)
	}

	if strings.Contains(key, "\x00") {
		return errors.New(ErrS3KeyContainsNullBytes)
	}

	return nil
}

// CompressImageToWebP decodes an image from srcReader and encodes it into WebP format
// using the specified quality (low -> 60, medium -> 75, high -> 85).
func CompressImageToWebP(srcReader io.Reader, quality string) ([]byte, error) {
	img, format, err := image.Decode(srcReader)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image (format: %s): %w", format, err)
	}

	var qualityScore float32
	switch strings.ToLower(quality) {
	case ImageQualityLow:
		qualityScore = 60
	case ImageQualityMedium:
		qualityScore = 75
	case ImageQualityHigh, "":
		qualityScore = 85
	default:
		qualityScore = 85
	}

	var buf bytes.Buffer
	err = webp.Encode(&buf, img, &webp.EncoderOptions{
		Quality: qualityScore,
		Method:  4,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to encode WebP: %w", err)
	}

	return buf.Bytes(), nil
}
