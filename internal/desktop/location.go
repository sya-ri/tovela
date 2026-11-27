package desktop

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/sya-ri/tovela/internal/locale"
)

type locationKey struct {
	project  string
	googleID string
}

type locationChoices struct {
	zones      map[string][]string
	latencies  map[string]time.Duration
	err        error
	retryAfter time.Time
}

func (v *view) loadLocations(key locationKey, show func(locationChoices)) {
	choices := v.locations[key]
	waiting := choices.err != nil && time.Now().Before(choices.retryAfter)
	if waiting || len(choices.latencies) > 0 {
		show(choices)
		return
	}

	v.run(func(ctx context.Context) error {
		choices.err = nil
		if len(choices.zones) == 0 {
			choices.zones, choices.err = v.service.RelayZones(ctx, key.project, key.googleID)
		}
		if choices.err == nil {
			zones := slices.Concat(slices.Collect(maps.Values(choices.zones))...)
			slices.Sort(zones)
			choices.latencies, choices.err = v.service.MeasureRelayZones(ctx, slices.Compact(zones))
		}
		if choices.err != nil {
			choices.retryAfter = time.Now().Add(30 * time.Second)
		}
		return nil
	}, func() {
		v.locations[key] = choices
		show(choices)
	})
}

// fastestZone uses the first zone in API order when latency measurements tie.
func fastestZone(zones []string, latencies map[string]time.Duration) string {
	var best string
	for _, zone := range zones {
		if latency := latencies[zone]; latency > 0 && (best == "" || latency < latencies[best]) {
			best = zone
		}
	}
	return best
}

// zonePicker keeps the displayed latency separate from the persisted zone ID.
type zonePicker struct {
	button    *zoneButton
	ping      *widget.Label
	content   *fyne.Container
	zone      string
	zones     []string
	latencies map[string]time.Duration
}

type zoneButton struct {
	widget.Button
	caption *widget.Label
}

func (b *zoneButton) AccessibilityLabel() string {
	return b.caption.Text
}

func newZonePicker(window fyne.Window, saved string) *zonePicker {
	picker := &zonePicker{ping: widget.NewLabelWithStyle("", fyne.TextAlignTrailing, fyne.TextStyle{})}
	caption := widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	caption.Truncation = fyne.TextTruncateEllipsis
	picker.button = &zoneButton{caption: caption}
	picker.button.ExtendBaseWidget(picker.button)
	picker.button.Alignment = widget.ButtonAlignTrailing
	picker.button.Icon = theme.MenuDropDownIcon()
	picker.button.OnTapped = func() { picker.open(window) }
	arrowSpace := theme.Size(theme.SizeNameInlineIcon) + theme.Size(theme.SizeNameInnerPadding)
	latency := container.New(layout.NewCustomPaddedLayout(0, 0, 8, 0), picker.ping)
	values := container.NewBorder(nil, nil, nil, latency, caption)
	picker.content = container.NewStack(picker.button, container.New(layout.NewCustomPaddedLayout(0, 0, 0, arrowSpace), values))
	if saved != "" {
		picker.zones = []string{saved}
	}
	picker.selectZone(saved)
	return picker
}

func (p *zonePicker) setOptions(zones []string, latencies map[string]time.Duration) {
	p.zones, p.latencies = zones, latencies
	if !slices.Contains(zones, p.zone) {
		p.zone = ""
	}
	p.selectZone(p.zone)
}

func (p *zonePicker) selected() string {
	return p.zone
}

func (p *zonePicker) selectZone(zone string) {
	if zone != "" && !slices.Contains(p.zones, zone) {
		return
	}
	p.zone = zone
	if zone == "" {
		p.button.caption.SetText(locale.T("Choose a zone"))
		p.ping.Hide()
		return
	}
	p.button.caption.SetText(zone)
	p.ping.SetText(formatLatency(p.latencies[zone]))
	p.ping.Show()
}

func formatLatency(duration time.Duration) string {
	if duration <= 0 {
		return "—"
	}
	return fmt.Sprintf("%d ms", max(1, duration.Round(time.Millisecond).Milliseconds()))
}

func (p *zonePicker) open(window fyne.Window) {
	if len(p.zones) == 0 {
		return
	}
	list := widget.NewList(func() int { return len(p.zones) }, func() fyne.CanvasObject {
		zone := widget.NewLabel("")
		zone.Truncation = fyne.TextTruncateEllipsis
		latency := widget.NewLabelWithStyle("", fyne.TextAlignTrailing, fyne.TextStyle{})
		return container.NewBorder(nil, nil, nil, latency, zone)
	}, func(index widget.ListItemID, item fyne.CanvasObject) {
		row := item.(*fyne.Container)
		zone := p.zones[index]
		row.Objects[0].(*widget.Label).SetText(zone)
		row.Objects[1].(*widget.Label).SetText(formatLatency(p.latencies[zone]))
	})
	d := dialog.NewCustom(locale.T("Relay location"), locale.T("Close"), list, window)
	d.Resize(fyne.NewSize(min(440, window.Canvas().Size().Width-32), min(320, window.Canvas().Size().Height-32)))
	d.Show()
	if selected := slices.Index(p.zones, p.zone); selected >= 0 {
		list.ScrollTo(selected)
	}
	list.OnSelected = func(index widget.ListItemID) {
		p.selectZone(p.zones[index])
		d.Hide()
	}
	window.Canvas().Focus(list)
}
