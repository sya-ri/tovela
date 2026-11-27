package desktop

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"slices"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/sya-ri/tovela/internal/locale"
	"github.com/sya-ri/tovela/internal/relay"
)

type backend interface {
	GetState() relay.DesktopState
	SelectProfile(string) error
	AddProfile() error
	SaveConfig(relay.Config) error
	LogIn(context.Context, string) error
	RelayZones(context.Context, string, string) ([]string, error)
	MeasureRelayZones(context.Context, []string) (map[string]time.Duration, error)
	Start(context.Context, bool) error
	Extend(context.Context) error
	Stop(context.Context) error
	ServiceShutdown() error
}

type view struct {
	window  fyne.Window
	service backend
	state   relay.DesktopState
	busy    bool
	closing bool
	problem string
	copied  string

	cancelAction context.CancelFunc
	actionDone   chan struct{}
	cleanupDone  chan struct{}

	profiles *widget.Select
	add      *widget.Button
	settings *widget.Button
	status   *widget.Label
	address  *widget.Label
	copy     *widget.Button
	udp      *widget.Label
	expiry   *widget.Label
	details  *fyne.Container
	error    *widget.Label
	replace  *widget.Check
	start    *widget.Button
	extend   *widget.Button
	stop     *widget.Button

	settingsChanged func()
	locations       map[locationKey]locationChoices
}

var phaseLabels = map[string]string{
	"idle": "Stopped", "starting": "Starting", "running": "Online",
	"stopping": "Stopping", "recoverable": "Reconnect needed",
}

// Run opens the desktop window and closes all active relays when the app exits.
func Run(app fyne.App, window fyne.Window, service *relay.Desktop) {
	v := newView(window, service, app.Icon())
	window.SetCloseIntercept(func() { v.close(app) })

	done := make(chan struct{})
	defer close(done)
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				fyne.Do(v.refresh)
			case <-done:
				return
			}
		}
	}()
	window.ShowAndRun()
	if v.cleanupDone != nil {
		<-v.cleanupDone
	} else if err := v.shutdown(); err != nil {
		log.Print(err)
	}
}

func newView(window fyne.Window, service backend, icon fyne.Resource) *view {
	v := &view{window: window, service: service, locations: make(map[locationKey]locationChoices)}
	logo := canvas.NewImageFromResource(icon)
	logo.FillMode = canvas.ImageFillContain
	logo.SetMinSize(fyne.NewSize(36, 36))
	v.profiles = &widget.Select{PlaceHolder: locale.T("Relay profile")}
	v.add = widget.NewButtonWithIcon("", theme.ContentAddIcon(), func() {
		v.run(func(context.Context) error { return service.AddProfile() }, func() {
			v.replace.SetChecked(false)
			v.openSettings()
		})
	})
	v.settings = widget.NewButtonWithIcon("", theme.SettingsIcon(), v.openSettings)
	header := container.NewBorder(nil, nil, logo, container.NewHBox(v.add, v.settings), v.profiles)

	v.status = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	v.address = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	v.address.SizeName = theme.SizeNameHeadingText
	v.address.Wrapping = fyne.TextWrapBreak
	v.copy = widget.NewButtonWithIcon(locale.T("Copy"), theme.ContentCopyIcon(), func() {
		address := v.state.Config.Domain
		window.Clipboard().SetContent(address)
		v.copied = address
		v.render()
	})
	v.udp = widget.NewLabel("")
	v.expiry = widget.NewLabel("")
	metrics := container.NewGridWithColumns(3,
		metric("TCP", widget.NewLabel("25565")), metric("UDP", v.udp), metric(locale.T("Expires"), v.expiry),
	)
	v.details = container.NewVBox(widget.NewSeparator(), metrics)
	address := container.NewBorder(nil, nil, nil, container.NewCenter(v.copy), v.address)
	card := canvas.NewRectangle(theme.Color(theme.ColorNameInputBackground))
	card.CornerRadius = 16

	v.error = widget.NewLabel("")
	v.error.Importance = widget.DangerImportance
	v.error.Wrapping = fyne.TextWrapWord

	v.replace = widget.NewCheck(locale.T("Replace existing DNS records"), nil)
	v.start = widget.NewButtonWithIcon(locale.T("Start"), theme.MediaPlayIcon(), func() {
		if !v.readyToStart() {
			v.openSettings()
			return
		}
		replace := v.replace.Checked
		v.replace.SetChecked(false)
		v.run(func(ctx context.Context) error { return service.Start(ctx, replace) }, nil)
	})
	v.start.Importance = widget.HighImportance
	v.extend = widget.NewButtonWithIcon(locale.T("Extend 4 h"), theme.HistoryIcon(), func() { v.run(service.Extend, nil) })
	v.stop = widget.NewButtonWithIcon(locale.T("Stop"), theme.MediaStopIcon(), func() { v.run(service.Stop, nil) })
	actions := container.NewHBox(layout.NewSpacer(), v.start, v.extend, v.stop)
	cardContent := container.NewVBox(v.status, address, v.details,
		widget.NewSeparator(), v.error, v.replace, actions)
	body := container.NewStack(card, container.New(layout.NewCustomPaddedLayout(16, 20, 20, 20), cardContent))
	content := container.New(layout.NewCustomPaddedVBoxLayout(16), header, body)
	window.SetContent(container.New(layout.NewCustomPaddedLayout(18, 18, 16, 16), container.NewVScroll(content)))
	v.state = service.GetState()
	v.render()
	v.profiles.OnChanged = v.selectProfile
	return v
}

func metric(name string, value *widget.Label) fyne.CanvasObject {
	label := widget.NewLabel(name)
	label.TextStyle.Bold = true
	return container.New(layout.NewCustomPaddedVBoxLayout(0), label, value)
}

func (v *view) selectProfile(label string) {
	i := slices.Index(v.profiles.Options, label)
	if i < 0 || v.state.Profiles[i].ID == v.state.ActiveProfile {
		return
	}
	id := v.state.Profiles[i].ID
	v.run(func(context.Context) error { return v.service.SelectProfile(id) }, func() {
		v.replace.SetChecked(false)
		v.copied = ""
		v.render()
	})
}

func (v *view) refresh() {
	if v.closing {
		return
	}
	state := v.service.GetState()
	if state.State == v.state.State && state.ActiveProfile == v.state.ActiveProfile && slices.Equal(state.Profiles, v.state.Profiles) {
		return
	}
	v.state = state
	v.render()
}

func (v *view) readyToStart() bool {
	state := v.state
	return state.Config.Domain != "" && state.Config.Zone != "" && state.GoogleReady && state.CloudflareReady
}

func (v *view) render() {
	v.renderProfiles()
	phase := v.state.Phase
	locked := v.busy || v.closing || phase == "starting" || phase == "stopping"
	ready := v.readyToStart()
	setEnabled(v.profiles, !v.busy && !v.closing)
	setEnabled(v.add, !v.busy && !v.closing)
	setEnabled(v.settings, !locked)

	status := locale.T(phaseLabels[phase])
	if v.closing {
		status = locale.T("Closing…")
	}
	v.status.SetText(status)
	switch phase {
	case "running":
		v.status.Importance = widget.SuccessImportance
	case "starting", "stopping", "recoverable":
		v.status.Importance = widget.WarningImportance
	default:
		v.status.Importance = widget.MediumImportance
	}
	v.status.Refresh()
	address := v.state.Config.Domain
	hasAddress := address != ""
	setVisible(v.status, hasAddress || v.closing)
	setVisible(v.details, hasAddress)
	setVisible(v.copy, hasAddress)
	if !hasAddress {
		address = locale.T("New relay")
	}
	v.address.SetText(address)
	copyLabel, copyIcon := "Copy", theme.ContentCopyIcon()
	if hasAddress && v.copied == address {
		copyLabel, copyIcon = "Copied", theme.ConfirmIcon()
	}
	v.copy.SetText(locale.T(copyLabel))
	v.copy.SetIcon(copyIcon)
	udp := ""
	if v.state.Config.UDPPort != 0 {
		udp = strconv.Itoa(v.state.Config.UDPPort)
	}
	v.udp.SetText(orDash(udp))
	expiry := ""
	if !v.state.Deadline.IsZero() {
		expiry = v.state.Deadline.Local().Format(locale.T("Jan 2, 15:04"))
	}
	v.expiry.SetText(orDash(expiry))

	startLabel := "Start"
	startIcon := theme.MediaPlayIcon()
	if phase == "recoverable" {
		startLabel = "Reconnect"
	}
	if !ready && !locked {
		startLabel = "Set up relay"
		startIcon = theme.SettingsIcon()
	}
	v.start.SetText(locale.T(startLabel))
	v.start.SetIcon(startIcon)
	setVisible(v.start, phase == "idle" || phase == "recoverable")
	setEnabled(v.start, !locked)
	setVisible(v.extend, phase == "running")
	setEnabled(v.extend, !locked)
	setVisible(v.stop, phase == "running" || phase == "recoverable")
	setEnabled(v.stop, !locked)
	setVisible(v.replace, ready && phase == "idle")
	setEnabled(v.replace, !locked)

	text := v.problem
	if text == "" {
		text = v.state.Problem
	}
	v.error.SetText(text)
	setVisible(v.error, text != "" && v.settingsChanged == nil)
	if v.settingsChanged != nil {
		v.settingsChanged()
	}
	v.window.Content().Refresh()
}

func (v *view) renderProfiles() {
	labels := make([]string, 0, len(v.state.Profiles))
	newRelays := 0
	active := -1
	for i, profile := range v.state.Profiles {
		label := profile.Domain
		if label == "" {
			newRelays++
			label = locale.T("New relay")
			if newRelays > 1 {
				label = fmt.Sprintf(locale.T("New relay (%d)"), newRelays)
			}
		}
		if profile.Phase != "idle" {
			label = fmt.Sprintf(locale.T("%s (%s)"), label, locale.T(phaseLabels[profile.Phase]))
		}
		labels = append(labels, label)
		if profile.ID == v.state.ActiveProfile {
			active = i
		}
	}
	if !slices.Equal(labels, v.profiles.Options) {
		v.profiles.Options = labels
		v.profiles.Refresh()
	}
	if active >= 0 && v.profiles.Selected != labels[active] {
		changed := v.profiles.OnChanged
		v.profiles.OnChanged = nil
		v.profiles.SetSelectedIndex(active)
		v.profiles.OnChanged = changed
	}
}

func (v *view) run(action func(context.Context) error, success func()) {
	if v.busy || v.closing {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	v.cancelAction, v.actionDone = cancel, done
	v.busy, v.problem = true, ""
	v.render()
	go func() {
		defer close(done)
		defer cancel()
		err := action(ctx)
		fyne.Do(func() {
			v.busy = false
			if v.closing {
				return
			}
			if err != nil {
				v.problem = err.Error()
			}
			v.state = v.service.GetState()
			v.render()
			if errors.Is(err, relay.ErrFrpcBlocked) {
				v.showProtectionHistory()
			}
			if err == nil && success != nil {
				success()
			}
		})
	}()
}

func (v *view) showProtectionHistory() {
	message := widget.NewLabel(locale.T("Review the frpc.exe detection in Windows Security."))
	d := dialog.NewCustomWithoutButtons(locale.T("Windows blocked frpc"), message, v.window)
	open := widget.NewButtonWithIcon(locale.T("Open Protection History"), theme.NavigateNextIcon(), func() {
		d.Hide()
		if err := fyne.CurrentApp().OpenURL(&url.URL{Scheme: "windowsdefender", Host: "history"}); err != nil {
			dialog.ShowError(err, v.window)
		}
	})
	open.Importance = widget.HighImportance
	d.SetButtons([]fyne.CanvasObject{widget.NewButton(locale.T("Close"), d.Hide), open})
	d.Show()
}

func (v *view) close(app fyne.App) {
	if v.closing {
		return
	}
	v.closing = true
	v.render()
	v.cleanupDone = make(chan struct{})
	go func() {
		err := v.shutdown()
		close(v.cleanupDone)
		fyne.Do(func() {
			if err == nil {
				app.Quit()
				return
			}
			d := dialog.NewError(err, v.window)
			d.SetOnClosed(app.Quit)
			d.Show()
		})
	}()
}

func (v *view) shutdown() error {
	if v.cancelAction != nil {
		v.cancelAction()
	}
	if v.actionDone != nil {
		<-v.actionDone
	}
	return v.service.ServiceShutdown()
}

func setVisible(object fyne.CanvasObject, visible bool) {
	if visible {
		object.Show()
	} else {
		object.Hide()
	}
}

func setEnabled(object fyne.Disableable, enabled bool) {
	if enabled {
		object.Enable()
	} else {
		object.Disable()
	}
}

func orDash(value string) string {
	if value == "" {
		return "—"
	}
	return value
}
