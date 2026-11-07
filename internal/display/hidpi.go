package display

import "fmt"

// Resolution is a logical desktop size, not an advertised OS mode.
type Resolution struct{ Width, Height int }

func (r Resolution) Title() string { return fmt.Sprintf("%d × %d", r.Width, r.Height) }

// RecommendedHiDPI keeps a useful desktop on 16:9 QHD and denser panels.
// Availability is resolved separately; a recommendation is never a mode.
func RecommendedHiDPI(nativeWidth, nativeHeight int) []Resolution {
	if nativeWidth < 2560 || nativeWidth*9 != nativeHeight*16 {
		return nil
	}
	return []Resolution{{2560, 1440}, {2304, 1296}, {2496, 1404}}
}

func (r Resolution) HiDPIMode(modes []Mode, refreshRate int) (Mode, bool) {
	for _, mode := range Resolutions(modes, true, refreshRate) {
		if mode.Width == r.Width && mode.Height == r.Height && mode.PixelWidth == 2*r.Width && mode.PixelHeight == 2*r.Height {
			return mode, true
		}
	}
	return Mode{}, false
}
