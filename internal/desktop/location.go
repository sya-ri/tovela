package desktop

import (
	"context"
	"fmt"
	"time"

	"fyne.io/fyne/v2/widget"
	"github.com/sya-ri/tovela/internal/locale"
)

type locationKey struct {
	project  string
	googleID string
}

type locationChoices struct {
	zones      []string
	latencies  map[string]time.Duration
	err        error
	retryAfter time.Time
}

func (v *view) loadLocations(key locationKey, retry bool, show func(locationChoices)) {
	choices := v.locations[key]
	waiting := choices.err != nil && (!retry || time.Now().Before(choices.retryAfter))
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
			choices.latencies, choices.err = v.service.MeasureRelayZones(ctx, choices.zones)
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
	selectBox   *widget.Select
	zoneByLabel map[string]string
}

func newZonePicker(saved string) *zonePicker {
	picker := &zonePicker{selectBox: &widget.Select{PlaceHolder: locale.T("Choose a zone")}}
	if saved != "" {
		picker.setOptions([]string{saved}, nil)
		picker.selectZone(saved)
	}
	return picker
}

func (p *zonePicker) setOptions(zones []string, latencies map[string]time.Duration) {
	selected := p.selected()
	labels := make([]string, 0, len(zones))
	p.zoneByLabel = make(map[string]string, len(zones))
	for _, zone := range zones {
		label := zone
		if duration := latencies[zone]; duration > 0 {
			label = fmt.Sprintf("%s   %d ms", zone, max(1, duration.Round(time.Millisecond).Milliseconds()))
		}
		labels = append(labels, label)
		p.zoneByLabel[label] = zone
	}
	p.selectBox.SetOptions(labels)
	p.selectBox.ClearSelected()
	p.selectZone(selected)
}

func (p *zonePicker) selected() string {
	return p.zoneByLabel[p.selectBox.Selected]
}

func (p *zonePicker) selectZone(zone string) {
	for label, id := range p.zoneByLabel {
		if id == zone {
			p.selectBox.SetSelected(label)
			return
		}
	}
}
