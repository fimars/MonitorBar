//go:build darwin

package display

import (
	"context"
	"os"
	"testing"
	"time"
)

// This opt-in check briefly changes the actual display and restores it.
func TestLiveHiDPI(t *testing.T) {
	if os.Getenv("MONITORBAR_LIVE_HIDPI") != "1" {
		t.Skip("set MONITORBAR_LIVE_HIDPI=1 to test the connected display")
	}
	m := Target()
	original, ok := m.CurrentMode()
	if !ok {
		t.Fatal("cannot read current mode")
	}
	t.Cleanup(func() {
		if err := m.Apply(original); err != nil {
			t.Errorf("restore original mode: %v", err)
		}
	})
	for _, size := range []Resolution{{2048, 1152}, {2560, 1440}} {
		chosen, ok := size.HiDPIMode(m.Modes(), original.RefreshRate)
		if !ok {
			t.Fatalf("%s HiDPI unavailable", size.Title())
		}
		if err := m.Apply(chosen); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		err := m.WaitForMode(ctx, chosen)
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("verified %s HiDPI rendered at %d × %d @ %d Hz", chosen.Title(), chosen.PixelWidth, chosen.PixelHeight, chosen.RefreshRate)
	}
}
