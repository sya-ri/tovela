package relay

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"sync"
	"testing"
	"time"
)

func TestRelayZonesListsMatchingZonesAcrossPages(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.Method != http.MethodGet || req.URL.Path != "/compute/v1/projects/example-project/aggregated/machineTypes" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
		}
		query := req.URL.Query()
		if query.Get("filter") != `name eq "e2-micro|e2-small|e2-medium|e2-standard-2"` || query.Get("returnPartialSuccess") != "true" {
			t.Fatalf("unexpected query: %s", query.Encode())
		}
		switch query.Get("pageToken") {
		case "":
			return reply(200, `{
				"items": {
					"zones/us-central1-f": {"machineTypes": [{"name": "e2-micro"}]},
					"zones/us-central1-a": {"warning": {"code": "NO_RESULTS_ON_PAGE"}},
					"zones/europe-west2-a": {"machineTypes": [{"name": "e2-small"}]},
					"zones/invalid/zone": {"machineTypes": [{"name": "e2-micro"}]}
				},
				"nextPageToken": "second+page"
			}`)
		case "second+page":
			return reply(200, `{
				"items": {
					"zones/us-central1-f": {"machineTypes": [{"name": "e2-micro"}]},
					"zones/southamerica-east1-b": {"machineTypes": [{"name": "e2-micro"}]},
					"zones/asia-northeast1-b": {"machineTypes": [{"name": "e2-micro"}]}
				}
			}`)
		default:
			t.Fatalf("unexpected page token: %q", query.Get("pageToken"))
			return nil, nil
		}
	})}
	gce := compute{client: client, config: Config{Project: "example-project"}}
	got, err := gce.relayZones(context.Background())
	want := []string{"asia-northeast1-b", "southamerica-east1-b", "us-central1-f"}
	if err != nil || !slices.Equal(got[DefaultMachineType], want) || !slices.Equal(got["e2-small"], []string{"europe-west2-a"}) || len(got) != 2 || calls != 2 {
		t.Fatalf("relayZones() = %v, %v after %d calls, want %v after 2 calls", got, err, calls, want)
	}
}

func TestMeasureRelayZonesProbesEachRegionOnce(t *testing.T) {
	zones := []string{"us-central1-f", "europe-west2-c", "europe-west2-b", "asia-northeast1-a"}
	delays := map[string]time.Duration{"us-central1": 70 * time.Millisecond, "europe-west2": 30 * time.Millisecond}
	var seen sync.Map
	got, err := measureRelayZones(context.Background(), zones, func(_ context.Context, region string) (time.Duration, error) {
		if _, duplicate := seen.LoadOrStore(region, true); duplicate {
			t.Errorf("probed %s twice", region)
		}
		if delay, ok := delays[region]; ok {
			return delay, nil
		}
		return 0, errors.New("unreachable")
	})
	if err != nil || len(got) != 3 || got["europe-west2-b"] != delays["europe-west2"] ||
		got["europe-west2-c"] != delays["europe-west2"] || got["us-central1-f"] != delays["us-central1"] {
		t.Fatalf("measureRelayZones() = %v, %v", got, err)
	}
}

func TestMeasureRelayZonesFailsWhenNoRegionResponds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	latencies, err := measureRelayZones(ctx, []string{"us-central1-f"}, func(ctx context.Context, _ string) (time.Duration, error) {
		return 0, ctx.Err()
	})
	if len(latencies) != 0 || err == nil {
		t.Fatalf("measureRelayZones() = %v, %v, want no measurements and an error", latencies, err)
	}
}
