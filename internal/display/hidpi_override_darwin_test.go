//go:build darwin

package display

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"
)

func TestCGSDescriptionLayout(t *testing.T) {
	var mode cgsModeDescription
	if unsafe.Sizeof(mode) != 212 || unsafe.Offsetof(mode.RefreshRate) != 190 || unsafe.Offsetof(mode.Density) != 208 {
		t.Fatalf("wrong CGS ABI: size %d, refresh %d, density %d", unsafe.Sizeof(mode), unsafe.Offsetof(mode.RefreshRate), unsafe.Offsetof(mode.Density))
	}
}

func TestHiDPIOverridePreservesAndRestores(t *testing.T) {
	for _, exists := range []bool{false, true} {
		name := "new display override"
		if exists {
			name = "existing display override"
		}
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "DisplayVendorID-4d67", "DisplayProductID-8788")
			original := []byte(`<?xml version="1.0"?><plist version="1.0"><dict><key>DisplayProductName</key><string>Original Panel</string><key>IODisplayEDID</key><data>AQID</data><key>scale-resolutions</key><array><data>AAAPAAAACHA=</data></array></dict></plist>`)
			if exists {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, original, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			presets := RecommendedHiDPI(2560, 1440)
			if err := updateHiDPIOverride(path, presets, false); err != nil {
				t.Fatal(err)
			}
			first, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, encoded := range []string{"AAAUAAAAC0A=", "AAASAAAACiA=", "AAATgAAACvg="} {
				if strings.Count(string(first), encoded) != 1 {
					t.Fatalf("expected one entry %s in %s", encoded, first)
				}
			}
			if exists {
				for _, key := range []string{"DisplayProductName", "IODisplayEDID"} {
					data, err := exec.Command("/usr/bin/plutil", "-extract", key, "raw", "-o", "-", path).Output()
					if err != nil {
						t.Fatal(err)
					}
					want := "Original Panel"
					if key == "IODisplayEDID" {
						want = "AQID"
					}
					if strings.TrimSpace(string(data)) != want {
						t.Fatalf("lost %s: %s", key, data)
					}
				}
				if !bytes.Contains(first, []byte("AAAPAAAACHA=")) {
					t.Fatal("lost another app's scaled mode")
				}
			}
			if err := updateHiDPIOverride(path, presets, false); err != nil {
				t.Fatal(err)
			}
			second, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(first, second) {
				t.Fatal("installing twice changed the override")
			}
			if err := updateHiDPIOverride(path, presets, true); err != nil {
				t.Fatal(err)
			}
			if exists {
				restored, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(restored, original) {
					t.Fatal("did not restore original bytes")
				}
			} else if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("new override not removed: %v", err)
			}
			if err := updateHiDPIOverride(path, presets, true); err != nil {
				t.Fatalf("restore twice: %v", err)
			}
		})
	}
}

func TestHiDPIOverrideRefusesExternalChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "panel")
	presets := RecommendedHiDPI(2560, 1440)
	if err := updateHiDPIOverride(path, presets, false); err != nil {
		t.Fatal(err)
	}
	external := []byte("another app changed the configuration")
	if err := os.WriteFile(path, external, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, remove := range []bool{false, true} {
		if err := updateHiDPIOverride(path, presets, remove); err == nil {
			t.Fatal("overwrote an external change")
		}
	}
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(current, external) {
		t.Fatal("external change was lost")
	}
	if _, err := os.Stat(path + ".monitorbar.json"); err != nil {
		t.Fatalf("lost original backup: %v", err)
	}
}
