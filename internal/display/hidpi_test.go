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
