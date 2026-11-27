package desktop

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/sya-ri/tovela/internal/locale"
	"github.com/sya-ri/tovela/internal/relay"
)

func (v *view) openSettings() {
	v.problem = ""
	config := v.state.Config
	domain := entry(config.Domain, "mc.example.com")
	project := entry(config.Project, "")
	tcp := entry(strconv.Itoa(config.TCPPort), "25565")
	udp := entry("", locale.T("UDP port"))
	udpEnabled := widget.NewCheck("UDP", nil)
	udpEnabled.SetChecked(config.UDPPort != 0)
	if udpEnabled.Checked {
		udp.SetText(strconv.Itoa(config.UDPPort))
	}
	hours := entry(strconv.Itoa(config.Hours), "4")
	picker := newZonePicker(v.window, config.Zone)
	zone := picker.button

	var zones []string
	var closed bool
	network := entry(config.Network, "default")
	cfZone := entry(config.CloudflareZone, locale.T("Automatic"))
	googleID := entry(config.GoogleClientID, locale.T("Client ID"))
	cfID := entry(config.CloudflareClientID, locale.T("Client ID"))

	inputs := []fyne.Disableable{domain, project, tcp, udpEnabled, hours, network, cfZone, googleID, cfID}
	readConfig := func() (relay.Config, error) {
		update := relay.Config{
			Project: project.Text, Zone: picker.selected(), Network: strings.TrimSpace(network.Text),
			Domain: domain.Text, CloudflareZone: cfZone.Text,
			GoogleClientID: googleID.Text, CloudflareClientID: cfID.Text,
		}
		for _, field := range []struct {
			entry   *widget.Entry
			invalid string
			value   *int
		}{
			{tcp, "Enter a valid local TCP port.", &update.TCPPort},
			{udp, "Enter a valid UDP port.", &update.UDPPort},
			{hours, "Enter a valid auto-delete interval.", &update.Hours},
		} {
			if field.entry == udp && !udpEnabled.Checked {
				continue
			}
			number, err := strconv.Atoi(strings.TrimSpace(field.entry.Text))
			if err != nil || (field.entry == udp && (number < 1 || number > 65535)) {
				return relay.Config{}, errors.New(locale.T(field.invalid))
			}
			*field.value = number
		}
		return update, nil
	}

	perform := func(provider string, saved func()) {
		editable := v.state.Phase == "idle"
		var draft relay.Config
		if editable {
			value, err := readConfig()
			if err != nil {
				v.problem = err.Error()
				v.render()
				return
			}
			draft = value
		}
		v.run(func(ctx context.Context) error {
			if editable {
				if err := v.service.SaveConfig(draft); err != nil {
					return err
				}
			}
			if provider != "" {
				return v.service.LogIn(ctx, provider)
			}
			return nil
		}, saved)
	}

	googleReady := func() bool {
		return v.state.GoogleReady && strings.TrimSpace(googleID.Text) == v.state.Config.GoogleClientID
	}
	canLoad := func() bool {
		return v.state.Phase == "idle" && googleReady() && strings.TrimSpace(project.Text) != ""
	}
	loadZones := func(after func()) {
		if closed || !canLoad() || v.busy || v.closing {
			return
		}
		key := locationKey{strings.TrimSpace(project.Text), strings.TrimSpace(googleID.Text)}
		v.loadLocations(key, func(choices locationChoices) {
			if closed {
				return
			}
			zones = choices.zones
			if len(zones) > 0 {
				latencies := choices.latencies
				picker.setOptions(zones, latencies)
				if picker.selected() == "" {
					picker.selectZone(fastestZone(zones, latencies))
				}
			}
			v.problem = ""
			if choices.err != nil && (len(zones) == 0 || picker.selected() == "") {
				v.problem = choices.err.Error()
			}
			v.render()
			if after != nil && picker.selected() != "" {
				after()
			}
		})
	}

	project.OnSubmitted = func(string) { loadZones(nil) }

	google := widget.NewButtonWithIcon("", googleIcon, func() {
		perform("google", func() {
			clear(v.locations)
			loadZones(nil)
		})
	})
	cloudflare := widget.NewButtonWithIcon("", cloudflareIcon, func() { perform("cloudflare", nil) })
	google.Alignment = widget.ButtonAlignLeading
	cloudflare.Alignment = widget.ButtonAlignLeading
	cloudflareButton := container.NewThemeOverride(cloudflare, cloudflareButtonTheme{cloudflare.Theme()})
	ports := container.NewGridWithColumns(2,
		container.NewVBox(widget.NewLabel("TCP"), tcp),
		container.NewVBox(udpEnabled, udp),
	)
	form := settingsForm(
		settingRow{label: "Relay"},
		settingRow{"Hostname", domain},
		settingRow{"Project ID", project},
		settingRow{"Relay location", picker.content},
		settingRow{"Auto-delete after (h)", hours},
		settingRow{label: "Network"},
		settingRow{"Local ports", ports},
		settingRow{label: "Accounts"},
		settingRow{"Google", google},
		settingRow{"Cloudflare", cloudflareButton},
	)
	advanced := widget.NewAccordion(widget.NewAccordionItem(locale.T("Advanced"), settingsForm(
		settingRow{"VPC network", network},
		settingRow{"Cloudflare zone", cfZone},
		settingRow{"Google OAuth client ID", googleID},
		settingRow{"Cloudflare OAuth client ID", cfID},
	)))
	message := widget.NewLabel("")
	message.Wrapping = fyne.TextWrapWord
	ok := widget.NewButtonWithIcon(locale.T("Save"), theme.ConfirmIcon(), nil)
	ok.Importance = widget.HighImportance
	closeButton := widget.NewButton(locale.T("Close"), nil)
	actions := container.NewHBox(layout.NewSpacer(), closeButton, ok)
	content := container.NewVScroll(container.NewVBox(form, advanced, message, actions))
	d := dialog.NewCustomWithoutButtons(locale.T("Settings"), container.NewClip(content), v.window)
	closeButton.OnTapped = d.Hide
	save := func() { perform("", d.Hide) }
	ok.OnTapped = func() {
		if picker.selected() == "" && canLoad() && len(zones) == 0 {
			loadZones(save)
			return
		}
		save()
	}

	v.settingsChanged = func() {
		editable := v.state.Phase == "idle" && !v.busy && !v.closing
		for _, input := range inputs {
			setEnabled(input, editable)
		}
		setVisible(udp, udpEnabled.Checked)
		setEnabled(udp, editable && udpEnabled.Checked)
		ready := googleReady()
		if picker.selected() == "" {
			placeholder := "Choose a zone"
			if !ready {
				placeholder = "Sign in to Google"
			}
			zone.caption.SetText(locale.T(placeholder))
		}
		setEnabled(zone, editable && ready && len(zones) > 0)
		v.updateLogin(google, googleID, ready)
		cfReady := v.state.CloudflareReady && strings.TrimSpace(cfID.Text) == v.state.Config.CloudflareClientID
		v.updateLogin(cloudflare, cfID, cfReady)
		setEnabled(ok, editable)
		text := v.problem
		message.Importance = widget.DangerImportance
		if v.busy {
			message.Importance = widget.LowImportance
			text = locale.T("Working…")
		}
		revealError := !v.busy && text != "" && message.Text != text
		message.SetText(text)
		setVisible(message, text != "")
		if revealError {
			content.ScrollToBottom()
		}
	}
	projectID, clientID := strings.TrimSpace(project.Text), strings.TrimSpace(googleID.Text)
	project.OnChanged = func(value string) {
		trimmed := strings.TrimSpace(value)
		if trimmed != projectID {
			projectID = trimmed
			zones = nil
			if projectID == config.Project && config.Zone != "" {
				picker.setOptions([]string{config.Zone}, nil)
				picker.selectZone(config.Zone)
			} else {
				picker.setOptions(nil, nil)
			}
		}
		v.settingsChanged()
	}
	googleID.OnChanged = func(value string) {
		trimmed := strings.TrimSpace(value)
		if trimmed != clientID {
			clientID = trimmed
			zones = nil
		}
		v.settingsChanged()
	}
	cfID.OnChanged = func(string) { v.settingsChanged() }
	udpEnabled.OnChanged = func(bool) { v.settingsChanged() }
	d.SetOnClosed(func() {
		closed = true
		v.settingsChanged = nil
		v.render()
	})
	v.settingsChanged()
	canvasSize := v.window.Canvas().Size()
	horizontalInset := min(64, max(24, canvasSize.Width-490))
	d.Resize(canvasSize.Subtract(fyne.NewSize(horizontalInset, 24)))
	d.Show()
	loadZones(nil)
}

func (v *view) updateLogin(btn *widget.Button, id *widget.Entry, ready bool) {
	label := "Sign in"
	if ready {
		label = "Connected"
	}
	btn.SetText(locale.T(label))
	setEnabled(btn, !v.busy && !v.closing && strings.TrimSpace(id.Text) != "")
}

type settingRow struct {
	label   string
	control fyne.CanvasObject
}

func settingsForm(rows ...settingRow) *fyne.Container {
	cells := make([]fyne.CanvasObject, 0, 2*len(rows))
	for i, row := range rows {
		if row.control != nil {
			cells = append(cells, widget.NewLabel(locale.T(row.label)), row.control)
			continue
		}

		heading := widget.NewLabelWithStyle(locale.T(row.label), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		heading.SizeName = theme.SizeNameSubHeadingText
		var label fyne.CanvasObject = heading
		if i > 0 {
			label = container.New(layout.NewCustomPaddedLayout(12, 0, 0, 0), heading)
		}
		cells = append(cells, label, layout.NewSpacer())
	}
	return container.New(layout.NewFormLayout(), cells...)
}

func entry(value, placeholder string) *widget.Entry {
	input := widget.NewEntry()
	input.SetText(value)
	input.SetPlaceHolder(placeholder)
	return input
}
