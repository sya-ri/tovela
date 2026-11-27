package desktop

import (
	"context"
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
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
	config := v.state.Config
	content, update := relayLoadPanel(config.RelayMemoryMiB())
	title := fmt.Sprintf("%s (%s)", locale.T("Relay load"), config.RelayMachineType())
	d := dialog.NewCustom(title, locale.T("Close"), container.NewClip(container.NewVScroll(content)), v.window)
	d.SetOnClosed(func() {
		cancel()
		v.closeLoad = nil
	})
	v.closeLoad = d.Hide
	size := v.window.Canvas().Size()
	d.Resize(fyne.NewSize(min(480, size.Width-32), min(396, size.Height-24)))
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

func relayLoadPanel(memoryMiB int) (fyne.CanvasObject, func(relay.Load, error)) {
	cpuPlot := newLoadPlot(100, "%")
	memoryPlot := newLoadPlot(float64(max(1, memoryMiB)), " MiB")
	cpu := widget.NewLabel("—")
	memory := widget.NewLabel("—")
	start, end := widget.NewLabel(""), widget.NewLabel("")
	timeAxis := container.New(layout.NewCustomPaddedLayout(0, 0, loadAxisInset-8, 0), container.NewHBox(start, layout.NewSpacer(), end))
	status := widget.NewLabel(locale.T("Loading…"))
	status.Wrapping = fyne.TextWrapWord
	row := func(name string, value *widget.Label) fyne.CanvasObject {
		label := widget.NewLabelWithStyle(name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		return container.NewHBox(label, layout.NewSpacer(), value)
	}
	content := container.NewVBox(
		row("CPU", cpu), cpuPlot.content,
		row(locale.T("Memory"), memory), memoryPlot.content,
		timeAxis, status,
	)

	latest := func(samples []relay.LoadSample, format string) string {
		if len(samples) == 0 {
			return "—"
		}
		sample := samples[len(samples)-1]
		return fmt.Sprintf(format, sample.Value, sample.At.Local().Format("15:04"))
	}
	update := func(load relay.Load, err error) {
		if !load.End.IsZero() {
			cpuPlot.update(load.CPUPercent, load.End)
			memoryPlot.update(load.MemoryMiB, load.End)
			cpu.SetText(latest(load.CPUPercent, "%.1f%%  (%s)"))
			memory.SetText(latest(load.MemoryMiB, "%.0f MiB  (%s)"))
			start.SetText(load.End.Add(-relay.LoadWindow).Local().Format("15:04"))
			end.SetText(load.End.Local().Format("15:04"))
		}
		status.Importance = widget.LowImportance
		message := ""
		if err != nil {
			status.Importance = widget.DangerImportance
			message = err.Error()
		} else if len(load.CPUPercent) == 0 || len(load.MemoryMiB) == 0 {
			message = locale.T("Waiting for Cloud Monitoring data.")
		}
		status.SetText(message)
		setVisible(status, message != "")
		content.Refresh()
	}
	return content, update
}
