package desktop

import (
	"context"
	"slices"
	"time"

	"fyne.io/fyne/v2/widget"
)

type locationKey struct {
	project  string
	googleID string
}

type locationChoices struct {
	zones      []string
	suggested  string
	err        error
	retryAfter time.Time
}

func (v *view) loadLocations(key locationKey, recommend, retry bool, show func(locationChoices)) {
	choices := v.locations[key]
	waiting := choices.err != nil && (!retry || time.Now().Before(choices.retryAfter))
	if waiting || (len(choices.zones) > 0 && (!recommend || choices.suggested != "")) {
		show(choices)
		return
	}

	v.run(func(ctx context.Context) error {
		choices.err = nil
		if len(choices.zones) == 0 {
			choices.zones, choices.err = v.service.RelayZones(ctx, key.project, key.googleID)
		}
		if choices.err == nil && recommend {
			choices.suggested, choices.err = v.service.FastestRelayZone(ctx, choices.zones)
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

func setLocationOptions(picker *widget.Select, zones []string) {
	picker.SetOptions(zones)
	if !slices.Contains(zones, picker.Selected) {
		picker.ClearSelected()
	}
}
