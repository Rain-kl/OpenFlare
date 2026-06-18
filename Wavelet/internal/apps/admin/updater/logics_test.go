// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package updater

import (
	"runtime"
	"testing"
	"time"
)

func TestParseRepository(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "short form", input: "Rain-kl/OpenFlare", want: "Rain-kl/OpenFlare"},
		{name: "GitHub URL", input: "https://github.com/Rain-kl/OpenFlare.git", want: "Rain-kl/OpenFlare"},
		{name: "unsupported host", input: "https://example.com/Rain-kl/OpenFlare", wantErr: true},
		{name: "missing owner", input: "OpenFlare", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRepository(tt.input)
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Errorf("parseRepository(%q) error = %v, want error presence = %t", tt.input, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("parseRepository(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSelectLatestRelease(t *testing.T) {
	assetNameV1 := expectedAssetName("v1.0.0")
	assetNameV2 := expectedAssetName("v2.0.0")
	releases := []githubRelease{
		{
			TagName:   "v1.0.0",
			Published: time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC),
			Assets: []releaseAsset{{
				Name:               assetNameV1,
				BrowserDownloadURL: "https://example.com/v1",
				State:              "uploaded",
			}},
		},
		{
			TagName:   "v2.0.0",
			Published: time.Date(2026, time.June, 2, 0, 0, 0, 0, time.UTC),
			Assets: []releaseAsset{{
				Name:               assetNameV2,
				BrowserDownloadURL: "https://example.com/v2",
				State:              "uploaded",
			}},
		},
		{
			TagName: "v3.0.0",
			Assets: []releaseAsset{{
				Name:               "wavelet_v3.0.0_other_platform.tar.gz",
				BrowserDownloadURL: "https://example.com/v3",
				State:              "uploaded",
			}},
		},
	}

	release, asset, err := selectLatestRelease("Rain-kl/OpenFlare", releases)
	if err != nil {
		t.Fatalf("selectLatestRelease() error = %v", err)
	}
	if release.TagName != "v2.0.0" {
		t.Errorf("selectLatestRelease() tag = %q, want %q", release.TagName, "v2.0.0")
	}
	if asset.Name != assetNameV2 {
		t.Errorf("selectLatestRelease() asset = %q, want %q", asset.Name, assetNameV2)
	}
}

func TestSelectLatestReleaseWithCustomRepo(t *testing.T) {
	extension := "tar.gz"
	if runtime.GOOS == "windows" {
		extension = "zip"
	}
	releases := []githubRelease{
		{
			TagName:   "v1.0.0",
			Published: time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC),
			Assets: []releaseAsset{{
				Name:               "wavelet_v1.0.0_" + runtime.GOOS + "_" + runtime.GOARCH + "." + extension,
				BrowserDownloadURL: "https://example.com/v1",
				State:              "uploaded",
			}},
		},
		{
			TagName:   "v2.0.0",
			Published: time.Date(2026, time.June, 2, 0, 0, 0, 0, time.UTC),
			Assets: []releaseAsset{{
				Name:               "PixezSync_v2.0.0_" + runtime.GOOS + "_" + runtime.GOARCH + "." + extension,
				BrowserDownloadURL: "https://example.com/v2",
				State:              "uploaded",
			}},
		},
	}

	release, asset, err := selectLatestRelease("Rain-kl/PixezSync", releases)
	if err != nil {
		t.Fatalf("selectLatestRelease() error = %v", err)
	}
	if release.TagName != "v2.0.0" {
		t.Errorf("selectLatestRelease() tag = %q, want %q", release.TagName, "v2.0.0")
	}
	expectedName := "PixezSync_v2.0.0_" + runtime.GOOS + "_" + runtime.GOARCH + "." + extension
	if asset.Name != expectedName {
		t.Errorf("selectLatestRelease() asset = %q, want %q", asset.Name, expectedName)
	}
}

func TestExpectedAssetName(t *testing.T) {
	extension := "tar.gz"
	if runtime.GOOS == "windows" {
		extension = "zip"
	}
	want := "wavelet_v1.2.3_" + runtime.GOOS + "_" + runtime.GOARCH + "." + extension
	if got := expectedAssetName("v1.2.3"); got != want {
		t.Errorf("expectedAssetName(%q) = %q, want %q", "v1.2.3", got, want)
	}
}
