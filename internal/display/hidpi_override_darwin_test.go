//go:build darwin

package display

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func overrideTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestHiDPIOverridePreservesAndRestores(t *testing.T) {
	for _, exists := range []bool{false, true} {
		name := "new display override"
		if exists {
			name = "existing display override"
		}
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(overrideTempDir(t), "DisplayVendorID-4d67", "DisplayProductID-8788")
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
	path := filepath.Join(overrideTempDir(t), "panel")
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

func TestHiDPIOverrideRestoresEmptyOriginal(t *testing.T) {
	path := filepath.Join(overrideTempDir(t), "panel")
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := updateHiDPIOverride(path, RecommendedHiDPI(2560, 1440), false); err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		if err := updateHiDPIOverride(path, nil, true); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil || len(data) != 0 {
			t.Fatalf("empty original must remain a file: %q, %v", data, err)
		}
	}
}

func TestHiDPIHelperRequest(t *testing.T) {
	for _, tc := range []struct {
		name  string
		args  []string
		valid bool
	}{
		{"install QHD", []string{"install", "19815", "34696", "2560", "1440"}, true},
		{"restore without dimensions", []string{"restore", "19815", "34696", "0", "0"}, true},
		{"restore unsupported panel", []string{"restore", "19815", "34696", "1920", "1080"}, true},
		{"install unsupported panel", []string{"install", "19815", "34696", "1920", "1080"}, false},
		{"install without dimensions", []string{"install", "19815", "34696", "0", "0"}, false},
		{"missing arguments", []string{"restore"}, false},
		{"extra arguments", []string{"restore", "1", "2", "0", "0", "extra"}, false},
		{"unknown action", []string{"delete", "1", "2", "0", "0"}, false},
		{"zero vendor", []string{"restore", "0", "2", "0", "0"}, false},
		{"zero product", []string{"restore", "1", "0", "0", "0"}, false},
		{"negative dimension", []string{"restore", "1", "2", "-1", "0"}, false},
		{"oversized identity", []string{"restore", "65536", "2", "0", "0"}, false},
		{"path injection", []string{"restore", "../1", "2", "0", "0"}, false},
		{"shell injection", []string{"install", "1; touch file", "2", "2560", "1440"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path, _, remove, err := parseHiDPIRequest(tc.args)
			if (err == nil) != tc.valid {
				t.Fatalf("valid = %v, error = %v", tc.valid, err)
			}
			if tc.valid && (path != filepath.Join(overrideDirectory, "DisplayVendorID-4d67", "DisplayProductID-8788") || remove != (tc.args[0] == "restore")) {
				t.Fatalf("wrong request target %q, restore %v", path, remove)
			}
		})
	}
	if os.Geteuid() != 0 {
		if err := RunHiDPIHelper([]string{"restore", "19815", "34696", "0", "0"}); err == nil || !strings.Contains(err.Error(), "administrator") {
			t.Fatalf("helper did not require root: %v", err)
		}
	}
}

func TestHiDPIOverrideRejectsMalformedBackup(t *testing.T) {
	for _, record := range []string{"{", "null", "{}", `{"original":null,"installedHash":[],"previousHash":[]}`, `{"original":42,"installedHash":null,"previousHash":null}`} {
		t.Run(record, func(t *testing.T) {
			path := filepath.Join(overrideTempDir(t), "panel")
			original := []byte("unchanged original")
			if err := os.WriteFile(path, original, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path+".monitorbar.json", []byte(record), 0o644); err != nil {
				t.Fatal(err)
			}
			for _, remove := range []bool{false, true} {
				if err := updateHiDPIOverride(path, RecommendedHiDPI(2560, 1440), remove); err == nil {
					t.Fatal("accepted malformed backup")
				}
			}
			for file, want := range map[string][]byte{path: original, path + ".monitorbar.json": []byte(record)} {
				got, err := os.ReadFile(file)
				if err != nil || !bytes.Equal(got, want) {
					t.Fatalf("changed %s: %q, %v", file, got, err)
				}
			}
		})
	}
}

func TestHiDPIOverrideRejectsRedirectedFiles(t *testing.T) {
	for _, target := range []string{"override", "backup", "parent", "override directory", "backup directory"} {
		t.Run(target, func(t *testing.T) {
			dir := overrideTempDir(t)
			outside := filepath.Join(dir, "outside")
			original := []byte("outside must not change")
			if err := os.WriteFile(outside, original, 0o644); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "panel")
			redirect := path
			switch target {
			case "backup", "backup directory":
				redirect += ".monitorbar.json"
			case "parent":
				redirect = filepath.Join(dir, "redirect")
				outside = dir
				path = filepath.Join(redirect, "panel")
			}
			if strings.HasSuffix(target, "directory") {
				if err := os.Mkdir(redirect, 0o755); err != nil {
					t.Fatal(err)
				}
			} else if err := os.Symlink(outside, redirect); err != nil {
				t.Fatal(err)
			}
			for _, remove := range []bool{false, true} {
				if err := updateHiDPIOverride(path, RecommendedHiDPI(2560, 1440), remove); err == nil {
					t.Fatal("accepted redirected path")
				}
			}
			got, err := os.ReadFile(filepath.Join(dir, "outside"))
			if err != nil || !bytes.Equal(got, original) {
				t.Fatalf("outside file changed: %q, %v", got, err)
			}
		})
	}
}

func TestHiDPIOverrideRecoversInterruptedWrites(t *testing.T) {
	for _, interrupted := range []string{"install before file replacement", "restore before backup removal"} {
		t.Run(interrupted, func(t *testing.T) {
			path := filepath.Join(overrideTempDir(t), "panel")
			original := []byte(`<?xml version="1.0"?><plist version="1.0"><dict><key>DisplayProductName</key><string>Original</string></dict></plist>`)
			if err := os.WriteFile(path, original, 0o644); err != nil {
				t.Fatal(err)
			}
			presets := RecommendedHiDPI(2560, 1440)
			if err := updateHiDPIOverride(path, presets, false); err != nil {
				t.Fatal(err)
			}
			if interrupted == "install before file replacement" {
				installed, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				backup := overrideBackup{Original: original, PreviousHash: sha256.Sum256(installed), InstalledHash: sha256.Sum256([]byte("next staged file"))}
				data, err := json.Marshal(backup)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path+".monitorbar.json", data, 0o644); err != nil {
					t.Fatal(err)
				}
				if err := updateHiDPIOverride(path, presets, false); err != nil {
					t.Fatalf("retry staged install: %v", err)
				}
			} else if err := os.WriteFile(path, original, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := updateHiDPIOverride(path, nil, true); err != nil {
				t.Fatalf("restore without eligible presets: %v", err)
			}
			got, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(got, original) {
				t.Fatalf("original not recovered: %q, %v", got, err)
			}
			if _, err := os.Stat(path + ".monitorbar.json"); !os.IsNotExist(err) {
				t.Fatalf("managed backup remained after restoration: %v", err)
			}
		})
	}
}
