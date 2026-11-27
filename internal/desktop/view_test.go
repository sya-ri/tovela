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
	"fyne.io/fyne/v2/widget"
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

func (b snapshotBackend) RelayZones(context.Context, string, string) ([]string, error) {
	return []string{"asia-northeast1-b", "europe-west2-a", "northamerica-northeast2-a", "southamerica-east1-b", "us-central1-f"}, nil
}

func (b snapshotBackend) FastestRelayZone(_ context.Context, zones []string) (string, error) {
	for _, zone := range zones {
		if zone == "europe-west2-a" {
			return zone, nil
		}
	}
	return "", errors.New("the loaded zones did not include the fastest zone")
}

func waitForAction(t *testing.T, v *view) {
	t.Helper()
	select {
	case <-v.actionDone:
	case <-time.After(5 * time.Second):
		t.Fatal("The relay action did not finish.")
	}
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
			config.Domain, config.Project, config.Zone = "play.example.com", "example-project", "northamerica-northeast2-a"
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
				waitForAction(t, v)
			case "frpc-blocked", "frpc-blocked-ja":
				v.showProtectionHistory()
			}
			test.AssertRendersToImage(t, screen.name+".png", window.Canvas())
			if screen.name == "settings" {
				checkAutoSelect(t, v)
			}
		})
	}
}

func TestInitialLocationUsesOneRequestAndMeasurement(t *testing.T) {
	a := test.NewTempApp(t)
	window := a.NewWindow("Tovela")
	defer window.Close()
	window.Resize(fyne.NewSize(610, 500))

	config := relay.DefaultConfig("google-client", "cloudflare-client")
	config.Domain, config.Project = "play.example.com", "example-project"
	b := &countedLocationBackend{snapshotBackend: snapshotBackend{state: relay.DesktopState{
		State: relay.State{Config: config, Phase: "idle", GoogleReady: true},
	}}}
	v := newView(window, b, theme.SettingsIcon())
	test.Tap(v.settings)
	waitForAction(t, v)
	zone := locationPicker(t, v)
	if zone.Selected != "europe-west2-a" {
		t.Fatalf("initial zone = %q, want europe-west2-a", zone.Selected)
	}

	zone.SetSelected("us-central1-f")
	test.Tap(settingsButton(t, v, "Auto-select"))
	if zone.Selected != "europe-west2-a" {
		t.Errorf("cached suggestion selected %q, want europe-west2-a", zone.Selected)
	}
	test.Tap(settingsButton(t, v, "Close"))
	test.Tap(v.settings)
	if v.busy {
		waitForAction(t, v)
		t.Error("reopening settings started a cloud request")
	}
	if b.lists != 1 || b.probes != 1 {
		t.Errorf("zone list requests = %d, latency measurements = %d, want one each", b.lists, b.probes)
	}
}

type countedLocationBackend struct {
	snapshotBackend
	lists  int
	probes int
}

func (b *countedLocationBackend) RelayZones(ctx context.Context, project, clientID string) ([]string, error) {
	b.lists++
	return b.snapshotBackend.RelayZones(ctx, project, clientID)
}

func (b *countedLocationBackend) FastestRelayZone(ctx context.Context, zones []string) (string, error) {
	b.probes++
	return b.snapshotBackend.FastestRelayZone(ctx, zones)
}

func checkAutoSelect(t *testing.T, v *view) {
	t.Helper()
	zone := locationPicker(t, v)
	if zone.Selected != "northamerica-northeast2-a" {
		t.Fatalf("saved zone reopened as %q", zone.Selected)
	}
	test.Tap(settingsButton(t, v, "Auto-select"))
	waitForAction(t, v)
	if zone.Selected != "europe-west2-a" {
		t.Errorf("Auto-select picked %q, want europe-west2-a", zone.Selected)
	}
}

func locationPicker(t *testing.T, v *view) *widget.Select {
	t.Helper()
	for _, object := range test.LaidOutObjects(v.window.Canvas().Overlays().Top()) {
		if selectBox, ok := object.(*widget.Select); ok && selectBox.PlaceHolder == "Choose a zone" {
			return selectBox
		}
	}
	t.Fatal("The relay zone selector is unavailable.")
	return nil
}

func settingsButton(t *testing.T, v *view, label string) *widget.Button {
	t.Helper()
	for _, object := range test.LaidOutObjects(v.window.Canvas().Overlays().Top()) {
		if button, ok := object.(*widget.Button); ok && button.Text == label && !button.Disabled() {
			return button
		}
	}
	t.Fatalf("The %q button is unavailable.", label)
	return nil
}

func TestBlockedClientKeepsCleanupError(t *testing.T) {
	a := test.NewTempApp(t)
	window := a.NewWindow("Tovela")
	defer window.Close()

	v := newView(window, snapshotBackend{}, theme.SettingsIcon())
	failure := errors.Join(relay.ErrFrpcBlocked, errors.New("The VM could not be deleted."))
	v.run(func(context.Context) error { return failure }, nil)

	waitForAction(t, v)
	if v.problem != failure.Error() {
		t.Errorf("relay failure = %q, want %q", v.problem, failure.Error())
	}
	if window.Canvas().Overlays().Top() == nil {
		t.Error("Protection History guidance was not shown.")
	}
}

func TestRelayLocationKeepsSavedZone(t *testing.T) {
	test.NewTempApp(t)
	saved := "asia-northeast1-b"
	picker := &widget.Select{Options: []string{saved}, PlaceHolder: "Choose a zone"}
	picker.SetSelected(saved)
	setLocationOptions(picker, []string{saved, "southamerica-east1-b"})
	if picker.Selected != saved {
		t.Errorf("saved location %q reopened as %q", saved, picker.Selected)
	}
	setLocationOptions(picker, []string{"europe-west2-a"})
	if picker.Selected != "" {
		t.Errorf("previous location %q survived a project change", picker.Selected)
	}
}
