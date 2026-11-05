package display

import "testing"

func TestRecommendedHiDPIModes(t *testing.T) {
	presets := RecommendedHiDPI(2560, 1440)
	if len(presets) != 3 || presets[0] != (Resolution{2560, 1440}) || presets[1] != (Resolution{2304, 1296}) || presets[2] != (Resolution{2496, 1404}) {
		t.Fatalf("unexpected presets: %v", presets)
	}
	modes := []Mode{
		{Width: 2560, Height: 1440, PixelWidth: 2560, PixelHeight: 1440, RefreshRate: 100},
		{Width: 2560, Height: 1440, PixelWidth: 5120, PixelHeight: 2880, RefreshRate: 60},
		{Width: 2560, Height: 1440, PixelWidth: 5120, PixelHeight: 2880, RefreshRate: 100},
	}
	mode, ok := presets[0].HiDPIMode(modes, 100)
	if !ok || !mode.HiDPI() || mode.RefreshRate != 100 {
		t.Fatalf("wrong mode: %+v, available %v", mode, ok)
	}
	if _, ok := presets[1].HiDPIMode(modes, 100); ok {
		t.Fatal("a missing size became a selectable mode")
	}
	if _, ok := presets[0].HiDPIMode(modes[:1], 100); ok {
		t.Fatal("LoDPI was offered as a HiDPI recommendation")
	}
	if got := RecommendedHiDPI(1920, 1080); len(got) != 0 {
		t.Fatalf("QHD recommendations on 1080p: %v", got)
	}
	if got := RecommendedHiDPI(3440, 1440); len(got) != 0 {
		t.Fatalf("16:9 recommendations on ultrawide: %v", got)
	}
}

func TestEDIDPanelSize(t *testing.T) {
	edid := make([]byte, 128)
	edid[21], edid[22] = 60, 33
	edid[54] = 1
	edid[56], edid[58] = 0, 0xa0
	edid[59], edid[61] = 0xa0, 0x50
	w, h, mmW, mmH := edidDimensions(edid)
	if w != 2560 || h != 1440 || mmW != 600 || mmH != 330 {
		t.Fatalf("dimensions: %d %d %d %d", w, h, mmW, mmH)
	}
	monitor := Monitor{NativeWidth: w, NativeHeight: h, PhysicalWidthMM: mmW, PhysicalHeightMM: mmH}
	current := Mode{Width: 4096, Height: 2304, PixelWidth: 4096, PixelHeight: 2304}
	if w, h := monitor.NativeSize([]Mode{current}, current); w != 2560 || h != 1440 {
		t.Fatalf("rendering was confused with physical panel: %d × %d", w, h)
	}
	if w, h, _, _ := edidDimensions(edid[:64]); w != 0 || h != 0 {
		t.Fatal("short EDID produced a native size")
	}
}
