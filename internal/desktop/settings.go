package desktop

import (
	"context"
	"errors"
	"fmt"
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
	v.replace.SetChecked(false)
	v.problem = ""
	config := v.state.Config
	domain := entry(config.Domain, "mc.example.com")
	project := entry(config.Project, locale.T("Project ID"))
	tcp := entry(strconv.Itoa(config.TCPPort), "25565")
	udp := entry("", locale.T("Optional"))
	if config.UDPPort != 0 {
		udp.SetText(strconv.Itoa(config.UDPPort))
	}
	hours := entry(strconv.Itoa(config.Hours), "4")
	zone := &widget.Select{PlaceHolder: locale.T("Choose a zone")}
	if config.Zone != "" {
		zone.SetOptions([]string{config.Zone})
		zone.SetSelected(config.Zone)
	}
	automatic := widget.NewButton(locale.T("Auto-select"), nil)
	location := container.NewBorder(nil, nil, nil, automatic, zone)
	var zones []string
	var closed bool
	network := entry(config.Network, "default")
	cfZone := entry(config.CloudflareZone, locale.T("Automatic"))
	googleID := entry(config.GoogleClientID, locale.T("Client ID"))
	cfID := entry(config.CloudflareClientID, locale.T("Client ID"))

	form := widget.NewForm(
		widget.NewFormItem(locale.T("Hostname"), domain),
		widget.NewFormItem(locale.T("Google Cloud project"), project),
		widget.NewFormItem(locale.T("Relay location"), location),
		widget.NewFormItem(locale.T("Local TCP port"), tcp),
		widget.NewFormItem(locale.T("UDP port"), udp),
		widget.NewFormItem(locale.T("Duration (h)"), hours),
	)
	advanced := widget.NewAccordion(widget.NewAccordionItem(locale.T("Advanced"), widget.NewForm(
		widget.NewFormItem(locale.T("VPC network"), network),
		widget.NewFormItem(locale.T("Cloudflare zone"), cfZone),
		widget.NewFormItem(locale.T("Google OAuth client ID"), googleID),
		widget.NewFormItem(locale.T("Cloudflare OAuth client ID"), cfID),
	)))
	inputs := []fyne.Disableable{domain, project, tcp, udp, hours, network, cfZone, googleID, cfID}
	readConfig := func() (relay.Config, error) {
		update := relay.Config{
			Project: project.Text, Zone: zone.Selected, Network: strings.TrimSpace(network.Text),
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
			{hours, "Enter a valid duration.", &update.Hours},
		} {
			text := strings.TrimSpace(field.entry.Text)
			if field.entry == udp && text == "" {
				continue
			}
			number, err := strconv.Atoi(text)
			if err != nil {
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
	loadZones := func(automatic bool, after func()) {
		if closed || !canLoad() || v.busy || v.closing {
			return
		}
		key := locationKey{strings.TrimSpace(project.Text), strings.TrimSpace(googleID.Text)}
		recommend := automatic || zone.Selected == ""
		v.loadLocations(key, recommend, automatic, func(choices locationChoices) {
			if closed {
				return
			}
			zones = choices.zones
			if len(zones) > 0 {
				setLocationOptions(zone, zones)
				if recommend && choices.suggested != "" {
					zone.SetSelected(choices.suggested)
				}
			}
			v.problem = ""
			if choices.err != nil && (len(zones) == 0 || recommend) {
				v.problem = choices.err.Error()
			}
			v.render()
			if after != nil && zone.Selected != "" {
				after()
			}
		})
	}

	automatic.OnTapped = func() { loadZones(true, nil) }
	project.OnSubmitted = func(string) { loadZones(false, nil) }

	google := widget.NewButtonWithIcon("Google", googleIcon, func() {
		perform("google", func() {
			clear(v.locations)
			loadZones(false, nil)
		})
	})
	cloudflare := widget.NewButtonWithIcon("Cloudflare", cloudflareIcon, func() { perform("cloudflare", nil) })
	message := widget.NewLabel("")
	message.Wrapping = fyne.TextWrapWord
	formContent := container.NewVScroll(container.NewVBox(form, advanced))
	signIn := container.NewGridWithColumns(2, google, cloudflare)
	ok := widget.NewButtonWithIcon(locale.T("Save"), theme.ConfirmIcon(), nil)
	ok.Importance = widget.HighImportance
	closeButton := widget.NewButton(locale.T("Close"), nil)
	footer := container.NewVBox(signIn, message, container.NewHBox(layout.NewSpacer(), closeButton, ok))
	content := container.NewBorder(nil, footer, nil, nil, formContent)
	d := dialog.NewCustomWithoutButtons(locale.T("Settings"), content, v.window)
	closeButton.OnTapped = d.Hide
	save := func() { perform("", d.Hide) }
	ok.OnTapped = func() {
		if zone.Selected == "" && canLoad() && len(zones) == 0 {
			loadZones(false, save)
			return
		}
		save()
	}

	v.settingsChanged = func() {
		editable := v.state.Phase == "idle" && !v.busy && !v.closing
		for _, input := range inputs {
			setEnabled(input, editable)
		}
		ready := googleReady()
		placeholder := "Choose a zone"
		if !ready {
			placeholder = "Sign in to Google"
		}
		if zone.PlaceHolder != locale.T(placeholder) {
			zone.PlaceHolder = locale.T(placeholder)
			zone.Refresh()
		}
		setEnabled(zone, editable && ready && len(zones) > 0)
		setEnabled(automatic, editable && canLoad())
		v.updateLogin(google, googleID, ready, "Google")
		cfReady := v.state.CloudflareReady && strings.TrimSpace(cfID.Text) == v.state.Config.CloudflareClientID
		v.updateLogin(cloudflare, cfID, cfReady, "Cloudflare")
		setEnabled(ok, editable)
		text := v.problem
		message.Importance = widget.DangerImportance
		if v.busy {
			message.Importance = widget.LowImportance
			text = locale.T("Working…")
		}
		message.SetText(text)
		setVisible(message, text != "")
	}
	projectID, clientID := strings.TrimSpace(project.Text), strings.TrimSpace(googleID.Text)
	project.OnChanged = func(value string) {
		trimmed := strings.TrimSpace(value)
		if trimmed != projectID {
			projectID = trimmed
			zones = nil
			if projectID == config.Project && config.Zone != "" {
				zone.SetOptions([]string{config.Zone})
				zone.SetSelected(config.Zone)
			} else {
				setLocationOptions(zone, nil)
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
	d.SetOnClosed(func() {
		closed = true
		v.settingsChanged = nil
		v.render()
	})
	v.settingsChanged()
	d.Resize(v.window.Canvas().Size().Subtract(fyne.NewSize(64, 64)))
	d.Show()
	loadZones(false, nil)
}

func (v *view) updateLogin(btn *widget.Button, id *widget.Entry, ready bool, provider string) {
	label := "Sign in to %s"
	if ready {
		label = "%s connected"
	}
	btn.SetText(fmt.Sprintf(locale.T(label), provider))
	setEnabled(btn, !v.busy && !v.closing && strings.TrimSpace(id.Text) != "")
}

func entry(value, placeholder string) *widget.Entry {
	input := widget.NewEntry()
	input.SetText(value)
	input.SetPlaceHolder(placeholder)
	return input
}
