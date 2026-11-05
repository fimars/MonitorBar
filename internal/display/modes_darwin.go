//go:build darwin

package display

import (
	"math"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// CGS includes scaled desktop modes omitted by CoreGraphics. The ABI is
// private; keep it optional and verify every switch through CoreGraphics.
type cgsModeDescription struct {
	Number, Flags              uint32
	Width, Height, Depth       uint32
	Reserved                   [42]uint32
	ReservedShort, RefreshRate uint16
	ReservedTail               [4]uint32
	Density                    float32
}

var (
	cgsGetNumberOfDisplayModes           func(uint32, *int32) int32
	cgsGetDisplayModeDescriptionOfLength func(uint32, int32, *cgsModeDescription, int32) int32
	cgsConfigureDisplayMode              func(uintptr, uint32, int32) int32
	cgsOnce                              sync.Once
)

func loadCGS() {
	load()
	cgsOnce.Do(func() {
		lib, err := purego.Dlopen("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			return
		}
		for fn, name := range map[any]string{
			&cgsGetNumberOfDisplayModes:           "CGSGetNumberOfDisplayModes",
			&cgsGetDisplayModeDescriptionOfLength: "CGSGetDisplayModeDescriptionOfLength",
			&cgsConfigureDisplayMode:              "CGSConfigureDisplayMode",
		} {
			if ptr, err := purego.Dlsym(lib, name); err == nil {
				purego.RegisterFunc(fn, ptr)
			}
		}
	})
}

func (m Monitor) ModeDiagnostics() []ModeInfo {
	loadCGS()
	var out []ModeInfo
	array := cgDisplayCopyAllDisplayModes(m.handle, modeOptions)
	if array != 0 {
		defer cfRelease(array)
		for i := range cfArrayGetCount(array) {
			ref := cfArrayGetValueAtIndex(array, i)
			mode := newMode(ref)
			mode.native = nativeMode{}
			out = append(out, ModeInfo{Mode: mode, Usable: cgDisplayModeIsUsableForDesktopGUI(ref), Source: "CoreGraphics", Flags: cgDisplayModeGetIOFlags(ref)})
		}
	}
	if cgsGetNumberOfDisplayModes == nil || cgsGetDisplayModeDescriptionOfLength == nil || cgsConfigureDisplayMode == nil {
		return out
	}
	var count int32
	if cgsGetNumberOfDisplayModes(m.handle, &count) != 0 {
		return out
	}
	for i := int32(0); i < count; i++ {
		var desc cgsModeDescription
		if cgsGetDisplayModeDescriptionOfLength(m.handle, i, &desc, int32(unsafe.Sizeof(desc))) != 0 {
			continue
		}
		density := float64(desc.Density)
		if density < 1 || density > 4 || math.IsNaN(density) || desc.Width == 0 || desc.Height == 0 {
			continue
		}
		mode := Mode{Width: int(desc.Width), Height: int(desc.Height), PixelWidth: int(math.Round(float64(desc.Width) * density)), PixelHeight: int(math.Round(float64(desc.Height) * density)), RefreshRate: int(desc.RefreshRate), native: nativeMode{cgsID: int32(desc.Number)}}
		if mode.RefreshRate == 0 {
			mode.RefreshRate = 60
		}
		out = append(out, ModeInfo{Mode: mode, Usable: desc.Flags&0x40000000 == 0, Source: "CGS", Flags: desc.Flags})
	}
	return out
}
