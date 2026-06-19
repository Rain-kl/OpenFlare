package observability

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseMemInfoValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		line string
		want int64
	}{
		{line: "MemTotal:       16384000 kB", want: 16384000},
		{line: "MemAvailable:   8192000 kB", want: 8192000},
		{line: "invalid", want: 0},
		{line: "MemTotal: not-a-number kB", want: 0},
	}

	for _, tt := range tests {
		if got := parseMemInfoValue(tt.line); got != tt.want {
			t.Fatalf("parseMemInfoValue(%q) = %d, want %d", tt.line, got, tt.want)
		}
	}
}

func TestShouldSkipDiskDevice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		device string
		want   bool
	}{
		{device: "", want: true},
		{device: "loop0", want: true},
		{device: "ram0", want: true},
		{device: "dm-0", want: true},
		{device: "sda", want: false},
		{device: "nvme0n1", want: false},
	}

	for _, tt := range tests {
		if got := shouldSkipDiskDevice(tt.device); got != tt.want {
			t.Fatalf("shouldSkipDiskDevice(%q) = %v, want %v", tt.device, got, tt.want)
		}
	}
}

func TestReadFirstLine(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "sample.txt")
	if err := os.WriteFile(path, []byte("  first line\nsecond line\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if got := ReadFirstLine(path); got != "first line\nsecond line" {
		t.Fatalf("ReadFirstLine() = %q, want trimmed first line content", got)
	}
	if got := ReadFirstLine(filepath.Join(dir, "missing.txt")); got != "" {
		t.Fatalf("ReadFirstLine(missing) = %q, want empty string", got)
	}
}

func TestStatFilesystem(t *testing.T) {
	t.Parallel()

	total, used := StatFilesystem(t.TempDir())
	if total <= 0 {
		t.Fatalf("StatFilesystem() total = %d, want > 0", total)
	}
	if used < 0 || used > total {
		t.Fatalf("StatFilesystem() used = %d, total = %d", used, total)
	}
}