//go:build windows

package display

func (m Monitor) ModeDiagnostics() []ModeInfo {
	var out []ModeInfo
	for _, mode := range m.Modes() {
		out = append(out, ModeInfo{Mode: mode, Usable: true, Source: "Windows"})
	}
	return out
}
