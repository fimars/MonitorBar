//go:build darwin

package display

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const overrideDirectory = "/Library/Displays/Contents/Resources/Overrides"

type overrideBackup struct {
	Original      []byte   `json:"original"`
	InstalledHash [32]byte `json:"installedHash"`
	PreviousHash  [32]byte `json:"previousHash"`
}

func (m Monitor) overridePath() string {
	load()
	return filepath.Join(overrideDirectory, fmt.Sprintf("DisplayVendorID-%x", cgDisplayVendorNumber(m.handle)), fmt.Sprintf("DisplayProductID-%x", cgDisplayModelNumber(m.handle)))
}

func (m Monitor) HasHiDPIOverride() bool {
	_, err := os.Lstat(m.overridePath() + ".monitorbar.json")
	return err == nil
}

// ConfigureHiDPIOverride requests elevation for the display model's override.
func (m Monitor) ConfigureHiDPIOverride(remove bool, nativeWidth, nativeHeight int) error {
	load()
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	action := "install"
	if remove {
		action = "restore"
	}
	args := []string{executable, "--hidpi-helper", action, strconv.FormatUint(uint64(cgDisplayVendorNumber(m.handle)), 10), strconv.FormatUint(uint64(cgDisplayModelNumber(m.handle)), 10), strconv.Itoa(nativeWidth), strconv.Itoa(nativeHeight)}
	for i, arg := range args {
		args[i] = "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
	}
	script := "do shell script " + strconv.Quote(strings.Join(args, " ")) + " with administrator privileges"
	output, err := exec.Command("/usr/bin/osascript", "-e", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("display: authorize HiDPI configuration: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// RunHiDPIHelper is the elevated entry point. Paths are derived from
// numeric display identifiers inside a fixed system directory.
func RunHiDPIHelper(args []string) error {
	path, presets, remove, err := parseHiDPIRequest(args)
	if err != nil {
		return err
	}
	if os.Geteuid() != 0 {
		return errors.New("display: administrator authorization required")
	}
	return updateHiDPIOverride(path, presets, remove)
}

func parseHiDPIRequest(args []string) (string, []Resolution, bool, error) {
	if len(args) != 5 || (args[0] != "install" && args[0] != "restore") {
		return "", nil, false, errors.New("display: invalid HiDPI request")
	}
	var numbers [4]int
	for i, arg := range args[1:] {
		n, err := strconv.ParseUint(arg, 10, 16)
		if err != nil || (i < 2 && n == 0) {
			return "", nil, false, errors.New("display: invalid display identity or dimensions")
		}
		numbers[i] = int(n)
	}
	presets := RecommendedHiDPI(numbers[2], numbers[3])
	if args[0] == "install" && len(presets) == 0 {
		return "", nil, false, errors.New("display: no HiDPI presets for this panel")
	}
	path := filepath.Join(overrideDirectory, fmt.Sprintf("DisplayVendorID-%x", numbers[0]), fmt.Sprintf("DisplayProductID-%x", numbers[1]))
	return path, presets, args[0] == "restore", nil
}

func updateHiDPIOverride(path string, presets []Resolution, remove bool) (err error) {
	statePath := path + ".monitorbar.json"
	for _, file := range []string{path, statePath} {
		if err := regularOverridePath(file); err != nil {
			return err
		}
	}
	current, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	var backup overrideBackup
	saved, stateErr := os.ReadFile(statePath)
	if stateErr == nil {
		var record struct {
			Original      json.RawMessage `json:"original"`
			InstalledHash []byte          `json:"installedHash"`
			PreviousHash  []byte          `json:"previousHash"`
		}
		if err := json.Unmarshal(saved, &record); err != nil {
			return fmt.Errorf("display: read override backup: %w", err)
		}
		if len(record.Original) == 0 || len(record.InstalledHash) != sha256.Size || len(record.PreviousHash) != sha256.Size {
			return errors.New("display: incomplete override backup; files preserved")
		}
		if err := json.Unmarshal(record.Original, &backup.Original); err != nil {
			return fmt.Errorf("display: read original override: %w", err)
		}
		copy(backup.InstalledHash[:], record.InstalledHash)
		copy(backup.PreviousHash[:], record.PreviousHash)
		if hash := sha256.Sum256(current); hash != backup.InstalledHash && hash != backup.PreviousHash && !bytes.Equal(current, backup.Original) {
			return errors.New("display: override changed outside MonitorBar; original backup preserved")
		}
	} else if !errors.Is(stateErr, os.ErrNotExist) {
		return stateErr
	} else {
		backup.Original = current
	}
	if remove {
		if errors.Is(stateErr, os.ErrNotExist) {
			return nil
		}
		if backup.Original == nil {
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		} else if err := writeOverrideFile(path, backup.Original); err != nil {
			return err
		}
		return os.Remove(statePath)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	stage, err := os.CreateTemp(filepath.Dir(path), ".monitorbar-*")
	if err != nil {
		return err
	}
	stagePath := stage.Name()
	defer func() {
		if e := os.Remove(stagePath); e != nil && !errors.Is(e, os.ErrNotExist) {
			err = errors.Join(err, e)
		}
	}()
	if err := stage.Close(); err != nil {
		return err
	}
	content := current
	if len(content) == 0 {
		content = []byte(`<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict/></plist>`)
	}
	if err := os.WriteFile(stagePath, content, 0o644); err != nil {
		return err
	}
	// plutil preserves all unrelated plist fields, including data and names.
	if err := exec.Command("/usr/bin/plutil", "-extract", "scale-resolutions", "xml1", "-o", "/dev/null", stagePath).Run(); err != nil {
		if output, err := exec.Command("/usr/bin/plutil", "-insert", "scale-resolutions", "-xml", "<array/>", stagePath).CombinedOutput(); err != nil {
			return fmt.Errorf("display: initialize scale resolutions: %w: %s", err, output)
		}
	}
	for _, preset := range presets {
		var data [8]byte
		binary.BigEndian.PutUint32(data[:4], uint32(2*preset.Width))
		binary.BigEndian.PutUint32(data[4:], uint32(2*preset.Height))
		encoded := base64.StdEncoding.EncodeToString(data[:])
		array, err := exec.Command("/usr/bin/plutil", "-extract", "scale-resolutions", "xml1", "-o", "-", stagePath).Output()
		if err != nil {
			return err
		}
		if strings.Contains(strings.Join(strings.Fields(string(array)), ""), encoded) {
			continue
		}
		if output, err := exec.Command("/usr/bin/plutil", "-insert", "scale-resolutions.0", "-data", encoded, stagePath).CombinedOutput(); err != nil {
			return fmt.Errorf("display: add %s: %w: %s", preset.Title(), err, output)
		}
	}
	installed, err := os.ReadFile(stagePath)
	if err != nil {
		return err
	}
	backup.PreviousHash, backup.InstalledHash = sha256.Sum256(current), sha256.Sum256(installed)
	state, err := json.Marshal(backup)
	if err != nil {
		return err
	}
	if err := writeOverrideFile(statePath, state); err != nil {
		return err
	}
	return writeOverrideFile(path, installed)
}

func regularOverridePath(path string) error {
	for p := filepath.Clean(path); ; p = filepath.Dir(p) {
		info, err := os.Lstat(p)
		switch {
		case errors.Is(err, os.ErrNotExist):
		case err != nil:
			return fmt.Errorf("display: inspect override path %s: %w", p, err)
		case p == path && !info.Mode().IsRegular(), p != path && !info.IsDir():
			return fmt.Errorf("display: override path is not a regular file or directory: %s", p)
		}
		if p == filepath.Dir(p) {
			return nil
		}
	}
}

func writeOverrideFile(path string, data []byte) (err error) {
	if err := regularOverridePath(path); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".monitorbar-write-*")
	if err != nil {
		return err
	}
	defer func() {
		if e := os.Remove(f.Name()); e != nil && !errors.Is(e, os.ErrNotExist) {
			err = errors.Join(err, e)
		}
	}()
	if _, err = f.Write(data); err != nil {
		return errors.Join(err, f.Close())
	}
	if err = f.Chmod(0o644); err != nil {
		return errors.Join(err, f.Close())
	}
	if err = f.Sync(); err != nil {
		return errors.Join(err, f.Close())
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
