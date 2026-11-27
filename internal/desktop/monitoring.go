package desktop

import (
	"context"
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/sya-ri/tovela/internal/locale"
	"github.com/sya-ri/tovela/internal/relay"
)

func (v *view) showRelayLoad() {
	if v.state.Phase != "running" || v.closeLoad != nil {
		return
	}
	profile := v.state.ActiveProfile
	ctx, cancel := context.WithCancel(context.Background())
	content, update := relayLoadPanel(v.state.Config.RelayMachineType())
	d := dialog.NewCustom(locale.T("Relay load"), locale.T("Close"), content, v.window)
	d.SetOnClosed(func() {
		cancel()
		v.closeLoad = nil
	})
	v.closeLoad = d.Hide
	d.Resize(fyne.NewSize(min(410, v.window.Canvas().Size().Width-32), d.MinSize().Height+24))
	d.Show()

	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for ctx.Err() == nil {
			load, err := v.service.RelayLoad(ctx, profile)
			fyne.Do(func() {
				if ctx.Err() == nil {
					update(load, err)
				}
			})
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func relayLoadPanel(machineType string) (fyne.CanvasObject, func(relay.Load, error)) {
	cpu := widget.NewLabel("—")
	memory := widget.NewLabel("—")
	status := widget.NewLabel(locale.T("Loading…"))
	status.Wrapping = fyne.TextWrapWord
	content := container.NewVBox(settingsForm(
		settingRow{"Machine type", widget.NewLabel(machineType)},
		settingRow{"CPU", cpu},
		settingRow{"Memory", memory},
	), status)
	update := func(load relay.Load, err error) {
		if load.CPUFraction != nil {
			cpu.SetText(fmt.Sprintf("%.1f%%  (%s)", *load.CPUFraction*100, load.CPUTime.Local().Format("15:04")))
		}
		if load.MemoryBytes != nil {
			memory.SetText(fmt.Sprintf("%.0f MiB  (%s)", float64(*load.MemoryBytes)/(1<<20), load.MemoryTime.Local().Format("15:04")))
		}
		status.Importance = widget.LowImportance
		message := ""
		if err != nil {
			status.Importance = widget.DangerImportance
			message = err.Error()
		} else if load.CPUFraction == nil || load.MemoryBytes == nil {
			message = locale.T("Waiting for Cloud Monitoring data.")
		}
		status.SetText(message)
		setVisible(status, message != "")
	}
	return content, update
}
