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
	zone := &widget.Select{Options: []string{"asia-northeast1-a", "asia-northeast1-b", "asia-northeast1-c"}, PlaceHolder: locale.T("Choose a zone")}
	zone.SetSelected(config.Zone)
	network := entry(config.Network, "default")
	cfZone := entry(config.CloudflareZone, locale.T("Automatic"))
	googleID := entry(config.GoogleClientID, locale.T("Client ID"))
	cfID := entry(config.CloudflareClientID, locale.T("Client ID"))

	form := widget.NewForm(
		widget.NewFormItem(locale.T("Hostname"), domain),
		widget.NewFormItem(locale.T("Google Cloud project"), project),
		widget.NewFormItem(locale.T("Local TCP port"), tcp),
		widget.NewFormItem(locale.T("UDP port"), udp),
		widget.NewFormItem(locale.T("Duration (h)"), hours),
	)
	advanced := widget.NewAccordion(widget.NewAccordionItem(locale.T("Advanced"), widget.NewForm(
		widget.NewFormItem(locale.T("Google Cloud zone"), zone),
		widget.NewFormItem(locale.T("VPC network"), network),
		widget.NewFormItem(locale.T("Cloudflare zone"), cfZone),
		widget.NewFormItem(locale.T("Google OAuth client ID"), googleID),
		widget.NewFormItem(locale.T("Cloudflare OAuth client ID"), cfID),
	)))
	inputs := []fyne.Disableable{domain, project, tcp, udp, hours, zone, network, cfZone, googleID, cfID}
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

	google := widget.NewButton("Google", func() { perform("google", nil) })
	cloudflare := widget.NewButton("Cloudflare", func() { perform("cloudflare", nil) })
	message := widget.NewLabel("")
	message.Wrapping = fyne.TextWrapWord
	formContent := container.NewVScroll(container.NewVBox(form, advanced, container.NewGridWithColumns(2, google, cloudflare)))
	ok := widget.NewButtonWithIcon(locale.T("Save"), theme.ConfirmIcon(), nil)
	ok.Importance = widget.HighImportance
	closeButton := widget.NewButton(locale.T("Close"), nil)
	footer := container.NewVBox(message, container.NewHBox(layout.NewSpacer(), closeButton, ok))
	content := container.NewBorder(nil, footer, nil, nil, formContent)
	d := dialog.NewCustomWithoutButtons(locale.T("Settings"), content, v.window)
	closeButton.OnTapped = d.Hide
	ok.OnTapped = func() { perform("", d.Hide) }

	v.settingsChanged = func() {
		editable := v.state.Phase == "idle" && !v.busy && !v.closing
		for _, input := range inputs {
			setEnabled(input, editable)
		}
		v.updateLogin(google, googleID, v.state.GoogleReady, "Google")
		v.updateLogin(cloudflare, cfID, v.state.CloudflareReady, "Cloudflare")
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
	googleID.OnChanged = func(string) { v.updateLogin(google, googleID, false, "Google") }
	cfID.OnChanged = func(string) { v.updateLogin(cloudflare, cfID, false, "Cloudflare") }
	d.SetOnClosed(func() {
		v.settingsChanged = nil
		v.render()
	})
	v.settingsChanged()
	d.Resize(v.window.Canvas().Size().Subtract(fyne.NewSize(64, 64)))
	d.Show()
}

func (v *view) updateLogin(btn *widget.Button, id *widget.Entry, ready bool, provider string) {
	btn.SetText(fmt.Sprintf(locale.T("Sign in to %s"), provider))
	btn.SetIcon(theme.LoginIcon())
	if ready {
		btn.SetText(fmt.Sprintf(locale.T("%s connected"), provider))
		btn.SetIcon(theme.ConfirmIcon())
	}
	setEnabled(btn, !v.busy && !v.closing && strings.TrimSpace(id.Text) != "")
}

func entry(value, placeholder string) *widget.Entry {
	input := widget.NewEntry()
	input.SetText(value)
	input.SetPlaceHolder(placeholder)
	return input
}
