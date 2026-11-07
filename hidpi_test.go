package main

import (
	"errors"
	"runtime"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/binaryify/MonitorBar/internal/display"
	"github.com/binaryify/MonitorBar/locales"
)

func TestHiDPIRecommendations(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("advanced Override controls require macOS")
	}
	p, _ := testPanel(t)
	s := p.s
	s.monitor = display.Monitor{Key: "qhd", Name: "H27D9", NativeWidth: 2560, NativeHeight: 1440}
	s.current = display.Mode{Width: 2048, Height: 1152, PixelWidth: 4096, PixelHeight: 2304, RefreshRate: 100}
	s.modes = []display.Mode{s.current, {Width: 2560, Height: 1440, PixelWidth: 5120, PixelHeight: 2880, RefreshRate: 100}}
	s.link, s.prefs.Tab = nil, "modes"
	tt := newTester(p)
	if s.advancedOpen || tt.HasText(locales.HiDPIRecommended()) || tt.HasText(locales.HiDPIInstall()) {
		t.Fatal("advanced operations must start collapsed")
	}
	if err := tt.Click(locales.ModesAdvanced()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, text := range []string{locales.HiDPIRecommended(), "2560 × 1440", "2304 × 1296", "2496 × 1404", "100 Hz", locales.HiDPINotAvailable(), locales.HiDPIInstall(), locales.HiDPIInstallNote(), locales.HiDPIRecoveryNote()} {
		if !tt.HasText(text) {
			t.Errorf("missing %q in %q", text, tt.Texts())
		}
	}
	if tt.HasText(locales.HiDPIRestore()) {
		t.Fatal("restore offered without a managed backup")
	}
	if err := tt.Click("2304 × 1296"); err != nil {
		t.Fatal(err)
	}
	if s.configuringDisplay != nil || s.notice != "" {
		t.Fatal("an unavailable preset started an operation")
	}
	if err := tt.Click(locales.ModesAdvanced()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if s.advancedOpen || tt.HasText(locales.HiDPIRecommended()) {
		t.Fatal("disclosure did not collapse")
	}
	tt.Key(0, ui.KeySpace)
	tt.Frame()
	if !s.advancedOpen || !tt.HasText(locales.HiDPIRecommended()) {
		t.Fatal("Space must open the advanced disclosure")
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if s.advancedOpen || tt.HasText(locales.HiDPIRecommended()) {
		t.Fatal("Enter must close the advanced disclosure")
	}
}

func TestHiDPIRestoreWithoutRecommendations(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("advanced Override controls require macOS")
	}
	for _, builtin := range []bool{false, true} {
		p, _ := testPanel(t)
		s := p.s
		s.monitor = display.Monitor{Key: "unsupported", NativeWidth: 1920, NativeHeight: 1080, Builtin: builtin}
		s.current = display.Mode{Width: 1920, Height: 1080, PixelWidth: 1920, PixelHeight: 1080, RefreshRate: 60}
		s.modes = []display.Mode{s.current}
		s.link, s.prefs.Tab = nil, "modes"
		s.advancedOpen, s.hasHiDPIOverride = true, true
		tt := newTester(p)
		if !tt.HasText(locales.HiDPIRestore()) || !tt.HasText(locales.HiDPIRecoveryNote()) || tt.HasText(locales.HiDPIInstall()) || tt.HasText("2304 × 1296") {
			t.Fatalf("restore must depend only on the backup: %q", tt.Texts())
		}
	}
}

func TestHiDPIStateBelongsToEachOutput(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("advanced Override controls require macOS")
	}
	p, _ := twoDisplays(t)
	s := p.s
	a, b := s.outputs[0], s.outputs[1]
	a.link, s.prefs.Tab = nil, "modes"
	a.hasHiDPIOverride = true
	tt := newTester(p)
	if err := tt.Click(locales.ModesAdvanced()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.advancedOpen || !tt.HasText(locales.HiDPIRestore()) {
		t.Fatal("display A did not expose its backup")
	}
	s.showDisplay(b)
	tt.Frame()
	tt.Key(0, ui.KeySpace)
	tt.Frame()
	if b.advancedOpen || b.hasHiDPIOverride || tt.HasText(locales.HiDPIRestore()) {
		t.Fatal("display B inherited A's disclosure, focus or backup state")
	}
	s.showDisplay(a)
	tt.Frame()
	if !a.advancedOpen || !tt.HasText(locales.HiDPIRestore()) {
		t.Fatal("display A lost its advanced state")
	}
}

func TestHiDPIAndModeOperationsExcludeEachOther(t *testing.T) {
	p, _ := twoDisplays(t)
	s := p.s
	a, b := s.outputs[0], s.outputs[1]
	a.link, s.prefs.Tab = nil, "modes"
	a.advancedOpen, a.hasHiDPIOverride = true, true
	s.configuringDisplay = b
	s.notice = "pending on B"
	s.applyMode(a.modes[0])
	for _, remove := range []bool{false, true} {
		s.configureHiDPI(remove)
	}
	if runtime.GOOS == "darwin" {
		tt := newTester(p)
		for _, label := range []string{locales.HiDPIInstall(), locales.HiDPIRestore()} {
			if !tt.HasText(label) {
				t.Fatalf("missing disabled action %q", label)
			}
			if err := tt.Click(label); err != nil {
				t.Fatal(err)
			}
		}
	}
	if s.configuringDisplay != b || s.notice != "pending on B" {
		t.Fatal("mode and Override operations must share one pending target across displays")
	}
}

func TestHiDPICompletionBelongsToItsDisplay(t *testing.T) {
	for _, name := range []string{"installed", "restored", "failed", "other selected", "removed", "replacement", "identity changed"} {
		t.Run(name, func(t *testing.T) {
			p, _ := twoDisplays(t)
			s := p.s
			a, b := s.outputs[0], s.outputs[1]
			monitor := a.monitor
			s.configuringDisplay, s.notice = a, "pending"
			var err error
			switch name {
			case "failed":
				err = errors.New("authorization canceled")
			case "other selected":
				s.showDisplay(b)
				s.notice = "display B notice"
			case "removed":
				s.outputs, s.output = []*output{b}, b
				s.notice = "display B notice"
			case "replacement":
				s.output = &output{monitor: monitor}
				s.outputs = []*output{s.output, b}
				s.notice = "replacement notice"
			case "identity changed":
				a.monitor.Name = "replacement"
				s.notice = "replacement notice"
			}
			selected, notice := s.output, s.notice
			s.hiDPIConfigured(a, monitor, name == "restored", err)
			if s.configuringDisplay != nil || s.output != selected {
				t.Fatal("completion did not release its slot or changed selection")
			}
			switch name {
			case "installed":
				if s.notice != locales.HiDPIInstalled() {
					t.Fatalf("installation notice %q", s.notice)
				}
			case "restored":
				if s.notice != locales.HiDPIRestored() {
					t.Fatalf("restoration notice %q", s.notice)
				}
			case "failed":
				if s.notice != locales.HiDPIConfigFailed(err.Error()) || !strings.Contains(s.notice, err.Error()) {
					t.Fatalf("failure detail lost in %q", s.notice)
				}
			default:
				if s.notice != notice {
					t.Fatalf("stale completion changed notice %q to %q", notice, s.notice)
				}
			}
		})
	}
}
