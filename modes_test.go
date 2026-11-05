package main

import (
	"strconv"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/binaryify/MonitorBar/internal/display"
)

func TestResolutionListsKeepHighestThree(t *testing.T) {
	p, _ := testPanel(t)
	p.s.prefs.Tab = "modes"
	p.s.link = nil
	for _, width := range []int{1600, 1280} {
		p.s.modes = append(p.s.modes, display.Mode{Width: width, Height: width * 9 / 16, PixelWidth: width, PixelHeight: width * 9 / 16, RefreshRate: 160})
	}
	tt := newTester(p)
	closedHeight := p.contentHeight
	for _, want := range []string{"HiDPI（清晰）", "标准", "3360 × 1890", "3200 × 1800", "2560 × 1440", "3840 × 2160", "显示其余 3 种", "显示其余 2 种"} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}
	for _, hidden := range []string{"常用分辨率", "全部分辨率", "1600 × 900", "1280 × 720", "渲染 3840×2160", "推荐 HiDPI"} {
		if tt.HasText(hidden) {
			t.Errorf("unexpected %q in the collapsed lists", hidden)
		}
	}
	if err := tt.Click("显示其余 3 种"); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		tt.Frame()
	}
	if !tt.HasText("渲染 3840×2160") || !tt.HasText("渲染 2560×1440") || !p.hiDPIModesOpen || p.standardModesOpen {
		t.Fatal("show more must extend only the HiDPI list")
	}
	tt.Key(0, ui.KeySpace)
	for range 3 {
		tt.Frame()
	}
	if p.hiDPIModesOpen || tt.HasText("渲染 3840×2160") || p.contentHeight != closedHeight {
		t.Fatal("Space must restore the original three rows and their height")
	}
	tt.Key(0, ui.KeyEnter)
	for range 3 {
		tt.Frame()
	}
	if !p.hiDPIModesOpen || !tt.HasText("渲染 3840×2160") {
		t.Fatal("Enter must extend the same list")
	}
	tt.Key(0, ui.KeySpace)
	for range 3 {
		tt.Frame()
	}
	if err := tt.Click("显示其余 2 种"); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		tt.Frame()
	}
	if !p.standardModesOpen || p.hiDPIModesOpen || !tt.HasText("1280 × 720") || tt.HasText("渲染 2560×1440") {
		t.Fatal("the standard list must extend independently")
	}
	p.s.switchingMode = true
	tt.Frame()
	if err := tt.Click("3360 × 1890"); err != nil {
		t.Fatal(err)
	}
	if p.s.notice != "" {
		t.Fatal("disabled resolution must not start a display operation")
	}
}

func TestShortResolutionListsNeedNoDisclosure(t *testing.T) {
	for _, count := range []int{0, 2, 3} {
		t.Run(strconv.Itoa(count)+" modes", func(t *testing.T) {
			p, _ := testPanel(t)
			p.s.prefs.Tab = "modes"
			p.s.link = nil
			p.s.modes = p.s.modes[:count]
			tt := newTester(p)
			if tt.HasText("显示其余") || tt.HasText("收起") {
				t.Fatal("lists with at most three sizes must not offer show more")
			}
			for _, mode := range p.s.modes {
				if !tt.HasText(mode.Title()) {
					t.Errorf("missing available mode %v", mode)
				}
			}
		})
	}
}

func TestAdvancedSettingsCardEdges(t *testing.T) {
	p, _ := testPanel(t)
	p.s.prefs.Tab = "modes"
	p.s.link = nil
	tt := newTester(p)
	r, ok := tt.Find("高级设置")
	if !ok {
		t.Fatal("missing advanced settings")
	}
	tt.ClickAt(15, r.Y+r.H/2)
	for range 3 {
		tt.Frame()
	}
	if !p.advancedOpen || !tt.HasText("显示器信息") || !tt.HasText("推荐 HiDPI") {
		t.Fatal("the full card must open display details and recommended sizes")
	}
	tt.Key(0, ui.KeySpace)
	for range 3 {
		tt.Frame()
	}
	if p.advancedOpen || tt.HasText("显示器信息") || tt.HasText("推荐 HiDPI") {
		t.Fatal("Space must hide advanced settings")
	}
}
