package desktop

import (
	"context"
	"errors"
	"image/color"
	"os"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
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

func (b snapshotBackend) MeasureRelayZones(_ context.Context, zones []string) (map[string]time.Duration, error) {
	latencies := make(map[string]time.Duration)
	available := map[string]time.Duration{
		"asia-northeast1-b":         180 * time.Millisecond,
		"europe-west2-a":            28 * time.Millisecond,
		"northamerica-northeast2-a": 91 * time.Millisecond,
		"us-central1-f":             114 * time.Millisecond,
	}
	for _, zone := range zones {
		if duration := available[zone]; duration > 0 {
			latencies[zone] = duration
		}
	}
	return latencies, nil
}

func waitForAction(t *testing.T, v *view) {
	t.Helper()
	select {
	case <-v.actionDone:
	case <-time.After(5 * time.Second):
		t.Fatal("The relay action did not finish.")
	}
}

type snapshotTheme struct {
	fyne.Theme
	variant fyne.ThemeVariant
}

func (th snapshotTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	return th.Theme.Color(name, th.variant)
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
		dark    bool
		narrow  bool
	}{
		{name: "setup", phase: "idle"},
		{name: "stopped", phase: "idle"},
		{name: "starting", phase: "starting"},
		{name: "online", phase: "running"},
		{name: "online-dark", phase: "running", dark: true},
		{name: "error", phase: "idle", problem: "Check your local Minecraft Java server on port 25565."},
		{name: "settings", phase: "idle"},
		{name: "settings-options", phase: "idle"},
		{name: "frpc-blocked", phase: "idle"},
		{name: "settings-ja", locale: "ja-JP", phase: "idle"},
		{name: "settings-udp-off", phase: "idle"},
		{name: "settings-narrow-ja", locale: "ja-JP", phase: "idle", narrow: true},
		{name: "settings-bottom-ja", locale: "ja-JP", phase: "idle"},
		{name: "frpc-blocked-ja", locale: "ja-JP", phase: "idle"},
		{name: "dns-conflict-ja", locale: "ja-JP", phase: "idle"},
	} {
		t.Run(screen.name, func(t *testing.T) {
			if err := locale.Use(screen.locale); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = locale.Use("en") })
			a := test.NewTempApp(t)
			variant := theme.VariantLight
			if screen.dark {
				variant = theme.VariantDark
			}
			test.ApplyTheme(t, snapshotTheme{Theme(), variant})
			window := a.NewWindow("Tovela")
			defer window.Close()
			size := fyne.NewSize(610, 500)
			if screen.narrow {
				size = fyne.NewSize(500, 410)
			}
			if screen.name == "settings-bottom-ja" {
				size.Height = 528
			}
			window.Resize(size)
			config := relay.DefaultConfig("google-client", "cloudflare-client")
			config.Domain, config.Project, config.Zone = "play.example.com", "example-project", "northamerica-northeast2-a"
			if screen.name != "settings-udp-off" {
				config.UDPPort = 24454
			}

			state := relay.DesktopState{
				ActiveProfile: "default",
				Profiles:      []relay.Profile{{ID: "default", Domain: config.Domain, Phase: screen.phase}},
				State: relay.State{
					Config: config, Phase: screen.phase, Problem: screen.problem,
					GoogleReady: true, CloudflareReady: true,
				},
			}
			if screen.name == "setup" {
				state.Config = relay.DefaultConfig("", "")
				state.Profiles[0].Domain = ""
				state.GoogleReady, state.CloudflareReady = false, false
			}
			if screen.phase == "running" {
				state.Deadline = time.Date(2026, 11, 27, 22, 15, 0, 0, time.Local)
			}
			v := newView(window, snapshotBackend{state: state}, fyne.NewStaticResource("icon.svg", icon))
			switch screen.name {
			case "settings", "settings-options", "settings-ja", "settings-narrow-ja", "settings-udp-off", "settings-bottom-ja":
				test.Tap(v.settings)
				waitForAction(t, v)
				switch screen.name {
				case "settings-options":
					picker := locationPicker(t, v)
					position := fyne.CurrentApp().Driver().AbsolutePositionForObject(picker)
					test.TapCanvas(window.Canvas(), position.AddXY(picker.Size().Width-16, picker.Size().Height/2))
					locationList(t, v)
				case "settings-bottom-ja":
					checkSettingsScroll(t, v)
				}
			case "frpc-blocked", "frpc-blocked-ja":
				v.showProtectionHistory()
			case "dns-conflict-ja":
				v.confirmDNSReplacement(config.Domain)
			}
			test.AssertRendersToImage(t, screen.name+".png", window.Canvas())
			switch screen.name {
			case "settings":
				if zone := locationPicker(t, v).AccessibilityLabel(); zone != config.Zone {
					t.Errorf("saved zone reopened as %q, want %q", zone, config.Zone)
				}
			case "setup":
				window.Canvas().Focus(v.start)
				v.start.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})
				if window.Canvas().Overlays().Top() == nil {
					t.Error("The setup action did not open Settings.")
				}
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
	if zone.AccessibilityLabel() != "europe-west2-a" {
		t.Fatalf("initial zone = %q, want europe-west2-a", zone.AccessibilityLabel())
	}
	test.Tap(zone)
	list := locationList(t, v)
	for range 5 {
		list.TypedKey(&fyne.KeyEvent{Name: fyne.KeyDown})
	}
	list.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})
	if zone.AccessibilityLabel() != "us-central1-f" {
		t.Errorf("manual zone = %q, want us-central1-f", zone.AccessibilityLabel())
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

func (b *countedLocationBackend) MeasureRelayZones(ctx context.Context, zones []string) (map[string]time.Duration, error) {
	b.probes++
	return b.snapshotBackend.MeasureRelayZones(ctx, zones)
}

func checkSettingsScroll(t *testing.T, v *view) {
	t.Helper()
	var settings *container.Scroll
	var signIn []*widget.Button
	for _, object := range test.LaidOutObjects(v.window.Canvas().Overlays().Top()) {
		switch control := object.(type) {
		case *container.Scroll:
			if settings == nil && control.Direction == container.ScrollVerticalOnly {
				settings = control
			}
		case *widget.Button:
			if control.Icon == googleIcon || control.Icon == cloudflareIcon {
				signIn = append(signIn, control)
			}
		}
	}
	if settings == nil || len(signIn) != 2 {
		t.Fatal("The scrollable settings or account rows are unavailable.")
	}

	driver := fyne.CurrentApp().Driver()
	buttons := append(signIn, settingsButton(t, v, locale.T("Save")))
	positions := make([]float32, len(buttons))
	for i, button := range buttons {
		positions[i] = driver.AbsolutePositionForObject(button).Y
	}
	settings.ScrollToBottom()
	for i, button := range buttons {
		if driver.AbsolutePositionForObject(button).Y >= positions[i] {
			t.Errorf("%q did not scroll with the settings form", button.Text)
		}
	}
}

func locationPicker(t *testing.T, v *view) *zoneButton {
	t.Helper()
	for _, object := range test.LaidOutObjects(v.window.Canvas().Overlays().Top()) {
		if button, ok := object.(*zoneButton); ok && !button.Disabled() {
			return button
		}
	}
	t.Fatal("The relay zone selector is unavailable.")
	return nil
}

func locationList(t *testing.T, v *view) *widget.List {
	t.Helper()
	for _, object := range test.LaidOutObjects(v.window.Canvas().Overlays().Top()) {
		if list, ok := object.(*widget.List); ok {
			return list
		}
	}
	t.Fatal("The relay zone list is unavailable.")
	return nil
}

type dnsStartBackend struct {
	snapshotBackend
	attempts []bool
}

func (b *dnsStartBackend) Start(_ context.Context, replace bool) error {
	b.attempts = append(b.attempts, replace)
	if !replace {
		return &relay.DNSConflict{Hostname: b.state.Config.Domain}
	}
	return nil
}

func TestDNSReplacementRequiresConfirmationOnStart(t *testing.T) {
	a := test.NewTempApp(t)
	window := a.NewWindow("Tovela")
	defer window.Close()
	window.Resize(fyne.NewSize(610, 500))

	config := relay.DefaultConfig("google-client", "cloudflare-client")
	config.Domain, config.Project, config.Zone = "play.example.com", "example-project", "europe-west2-a"
	b := &dnsStartBackend{snapshotBackend: snapshotBackend{state: relay.DesktopState{
		ActiveProfile: "default",
		State:         relay.State{Config: config, Phase: "idle", GoogleReady: true, CloudflareReady: true},
	}}}
	v := newView(window, b, theme.SettingsIcon())
	test.Tap(v.start)
	waitForAction(t, v)
	test.Tap(settingsButton(t, v, "Cancel"))
	if window.Canvas().Overlays().Top() != nil || len(b.attempts) != 1 || b.attempts[0] {
		t.Fatalf("cancelled DNS confirmation produced attempts %v", b.attempts)
	}

	test.Tap(v.start)
	waitForAction(t, v)
	test.Tap(settingsButton(t, v, "Replace and start"))
	waitForAction(t, v)
	if len(b.attempts) != 3 || b.attempts[1] || !b.attempts[2] {
		t.Errorf("confirmed DNS replacement produced attempts %v, want [false false true]", b.attempts)
	}
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

type settingsSaveBackend struct {
	snapshotBackend
	saves int
}

func (b *settingsSaveBackend) SaveConfig(config relay.Config) error {
	b.saves++
	b.state.Config = config
	return nil
}

func TestUDPToggleControlsPortSaving(t *testing.T) {
	a := test.NewTempApp(t)
	window := a.NewWindow("Tovela")
	defer window.Close()
	window.Resize(fyne.NewSize(610, 500))

	config := relay.DefaultConfig("", "")
	config.Domain, config.Project, config.Zone = "play.example.com", "example-project", "europe-west2-a"
	config.UDPPort = 24454
	b := &settingsSaveBackend{snapshotBackend: snapshotBackend{state: relay.DesktopState{
		State: relay.State{Config: config, Phase: "idle"},
	}}}
	v := newView(window, b, theme.SettingsIcon())
	test.Tap(v.settings)

	toggle, port := udpControls(t, v)
	if !toggle.Checked || port.Text != "24454" {
		t.Fatalf("saved UDP settings reopened as enabled=%t, port=%q", toggle.Checked, port.Text)
	}
	test.Tap(toggle)
	if port.Visible() || !port.Disabled() {
		t.Error("disabled UDP port remains available")
	}
	test.Tap(settingsButton(t, v, "Save"))
	waitForAction(t, v)
	if b.state.Config.UDPPort != 0 {
		t.Errorf("disabled UDP saved port %d, want 0", b.state.Config.UDPPort)
	}

	test.Tap(v.settings)
	toggle, port = udpControls(t, v)
	test.Tap(toggle)
	port.SetText("0")
	test.Tap(settingsButton(t, v, "Save"))
	if b.saves != 1 || v.problem == "" {
		t.Fatal("enabled UDP accepted an invalid port")
	}
	port.SetText("19132")
	test.Tap(settingsButton(t, v, "Save"))
	waitForAction(t, v)
	if b.state.Config.UDPPort != 19132 {
		t.Errorf("enabled UDP saved port %d, want 19132", b.state.Config.UDPPort)
	}
}

func udpControls(t *testing.T, v *view) (*widget.Check, *widget.Entry) {
	t.Helper()
	var toggle *widget.Check
	var port *widget.Entry
	for _, object := range test.LaidOutObjects(v.window.Canvas().Overlays().Top()) {
		switch control := object.(type) {
		case *widget.Check:
			if control.Text == "UDP" {
				toggle = control
			}
		case *widget.Entry:
			if control.PlaceHolder == "UDP port" {
				port = control
			}
		}
	}
	if toggle == nil || port == nil {
		t.Fatal("The UDP toggle or port field is unavailable.")
	}
	return toggle, port
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
	picker := newZonePicker(nil, saved)
	picker.setOptions([]string{saved, "southamerica-east1-b"}, map[string]time.Duration{saved: 103 * time.Millisecond})
	if picker.selected() != saved || picker.ping.Text != "103 ms" {
		t.Errorf("saved location %q reopened as %q with ping %q", saved, picker.selected(), picker.ping.Text)
	}
	picker.setOptions([]string{"europe-west2-a"}, nil)
	if picker.selected() != "" {
		t.Errorf("previous location %q survived a project change", picker.selected())
	}
}
