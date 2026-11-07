//go:build windows

package display

import "errors"

func (m Monitor) HasHiDPIOverride() bool { return false }
func (m Monitor) ConfigureHiDPIOverride(bool, int, int) error {
	return errors.New("display: HiDPI presets require macOS")
}
func RunHiDPIHelper([]string) error { return errors.New("display: HiDPI presets require macOS") }
