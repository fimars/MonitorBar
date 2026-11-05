package main

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/binaryify/MonitorBar/internal/display"
)

func TestHiDPIRecommendations(t *testing.T) {
	p, _ := testPanel(t)
	p.s.monitor = display.Monitor{Name: "H27D9", NativeWidth: 2560, NativeHeight: 1440, PhysicalWidthMM: 600, PhysicalHeightMM: 330}
	p.s.current = display.Mode{Width: 2048, Height: 1152, PixelWidth: 4096, PixelHeight: 2304, RefreshRate: 100}
	p.s.modes = []display.Mode{p.s.current, {Width: 2560, Height: 1440, PixelWidth: 5120, PixelHeight: 2880, RefreshRate: 100}}
	p.s.link = nil
	p.s.prefs.Tab = "modes"
	tt := newTester(p)
	for _, text := range []string{"桌面空间", "面板 2560×1440", "渲染 4096×2304", "27″ · 109 PPI", "推荐 HiDPI", "2560 × 1440", "2304 × 1296", "2496 × 1404", "HiDPI · 100 Hz", "系统尚未提供", "添加缺失档位…"} {
		if !tt.HasText(text) {
			t.Errorf("missing %q in %q", text, tt.Texts())
		}
	}
	if tt.HasText("恢复原显示器配置…") {
		t.Error("restore offered before installing an override")
	}
	p.s.hasHiDPIOverride = true
	tt.Frame()
	if !tt.HasText("恢复原显示器配置…") {
		t.Error("no way to restore the installed configuration")
	}
	if dir := os.Getenv("MONITORBAR_SNAPSHOTS"); dir != "" {
		tt.SetScale(2)
		tt.Frame()
		f, err := os.Create(filepath.Join(dir, "hidpi-qhd.png"))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
