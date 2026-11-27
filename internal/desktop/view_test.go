package desktop

import (
	"context"
	"errors"
	"image/color"
	"os"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"github.com/sya-ri/tovela/internal/locale"
	"github.com/sya-ri/tovela/internal/relay"
)

type snapshotBackend struct {
	backend
	state relay.DesktopState
}

func (b snapshotBackend) GetState() relay.DesktopState {
	return b.state
}

type lightTheme struct {
	fyne.Theme
}

func (th lightTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	return th.Theme.Color(name, theme.VariantLight)
}

func TestRelayScreens(t *testing.T) {
	icon, err := os.ReadFile("../../docs/brand/icon.svg")
	if err != nil {
		t.Fatal(err)
	}
	for _, screen := range []struct {
		name    string
		locale  string
		phase   string
		problem string
	}{
		{name: "stopped", phase: "idle"},
		{name: "starting", phase: "starting"},
		{name: "online", phase: "running"},
		{name: "error", phase: "idle", problem: "Check your local Minecraft Java server on port 25565."},
		{name: "settings", phase: "idle"},
		{name: "frpc-blocked", phase: "idle"},
		{name: "settings-ja", locale: "ja-JP", phase: "idle"},
		{name: "frpc-blocked-ja", locale: "ja-JP", phase: "idle"},
	} {
		t.Run(screen.name, func(t *testing.T) {
			if err := locale.Use(screen.locale); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = locale.Use("en") })
			a := test.NewTempApp(t)
			test.ApplyTheme(t, lightTheme{Theme()})
			window := a.NewWindow("Tovela")
			defer window.Close()
			window.Resize(fyne.NewSize(610, 500))
			config := relay.DefaultConfig("google-client", "cloudflare-client")
			config.Domain, config.Project = "play.example.com", "example-project"
			config.UDPPort = 24454

			state := relay.DesktopState{
				ActiveProfile: "default",
				Profiles:      []relay.Profile{{ID: "default", Domain: config.Domain, Phase: screen.phase}},
				State: relay.State{
					Config: config, Phase: screen.phase, Problem: screen.problem,
					GoogleReady: true, CloudflareReady: true,
				},
			}
			if screen.phase == "running" {
				state.Deadline = time.Date(2026, 11, 27, 22, 15, 0, 0, time.Local)
			}
			v := newView(window, snapshotBackend{state: state}, fyne.NewStaticResource("icon.svg", icon))
			switch screen.name {
			case "settings", "settings-ja":
				test.Tap(v.settings)
			case "frpc-blocked", "frpc-blocked-ja":
				v.showProtectionHistory()
			}
			test.AssertRendersToImage(t, screen.name+".png", window.Canvas())
		})
	}
}

func TestBlockedClientKeepsCleanupError(t *testing.T) {
	a := test.NewTempApp(t)
	window := a.NewWindow("Tovela")
	defer window.Close()

	v := newView(window, snapshotBackend{}, theme.SettingsIcon())
	failure := errors.Join(relay.ErrFrpcBlocked, errors.New("The VM could not be deleted."))
	v.run(func(context.Context) error { return failure }, nil)

	select {
	case <-v.actionDone:
	case <-time.After(5 * time.Second):
		t.Fatal("The relay action did not finish.")
	}
	if v.problem != failure.Error() {
		t.Errorf("relay failure = %q, want %q", v.problem, failure.Error())
	}
	if window.Canvas().Overlays().Top() == nil {
		t.Error("Protection History guidance was not shown.")
	}
}
