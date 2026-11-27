package relay

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/sya-ri/tovela/internal/locale"
)

func validZone(zone string) bool {
	return len(zone) <= 63 && zonePattern.MatchString(zone)
}

// RelayZones lists zones where the selected project can see supported relay machine types.
func (d *Desktop) RelayZones(ctx context.Context, project, googleID string) (map[string][]string, error) {
	config := Config{Project: strings.TrimSpace(project), GoogleClientID: strings.TrimSpace(googleID)}
	if !projectPattern.MatchString(config.Project) {
		return nil, errors.New(locale.T("Enter a valid Google Cloud project ID."))
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	client, err := authenticatedClient(ctx, config, "google")
	if err != nil {
		return nil, err
	}
	zones, err := (compute{client: client, config: config}).relayZones(ctx)
	if err != nil {
		return nil, fmt.Errorf(locale.T("Could not load relay zones: %w"), err)
	}
	if len(zones) == 0 {
		return nil, errors.New(locale.T("Google Cloud returned no zones with the relay machine type."))
	}
	return zones, nil
}

func (c compute) relayZones(ctx context.Context) (map[string][]string, error) {
	query := url.Values{
		"filter":               {fmt.Sprintf("name eq %q", strings.Join(MachineTypes(), "|"))},
		"returnPartialSuccess": {"true"},
	}
	found := make(map[string][]string)
	supported := MachineTypes()
	for {
		var page struct {
			Items         map[string]struct{ MachineTypes []struct{ Name string } }
			NextPageToken string
		}
		address := c.projectEndpoint() + "/aggregated/machineTypes?" + query.Encode()
		if err := request(ctx, c.client, http.MethodGet, address, nil, &page); err != nil {
			return nil, err
		}
		for scope, entry := range page.Items {
			zone, ok := strings.CutPrefix(scope, "zones/")
			if !ok || !validZone(zone) {
				continue
			}
			for _, machine := range entry.MachineTypes {
				if slices.Contains(supported, machine.Name) {
					found[machine.Name] = append(found[machine.Name], zone)
				}
			}
		}
		if page.NextPageToken == "" {
			break
		}
		query.Set("pageToken", page.NextPageToken)
	}
	for machine, zones := range found {
		slices.Sort(zones)
		found[machine] = slices.Compact(zones)
	}
	return found, nil
}

// MeasureRelayZones estimates latency from this computer using regional Google endpoints.
func (d *Desktop) MeasureRelayZones(ctx context.Context, zones []string) (map[string]time.Duration, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	return measureRelayZones(ctx, zones, probeRegion)
}

func measureRelayZones(ctx context.Context, zones []string, probe func(context.Context, string) (time.Duration, error)) (map[string]time.Duration, error) {
	regions := make(map[string][]string)
	for _, zone := range zones {
		if validZone(zone) {
			region := zone[:len(zone)-2]
			regions[region] = append(regions[region], zone)
		}
	}

	type measurement struct {
		region   string
		duration time.Duration
	}
	results := make(chan measurement, len(regions))
	for region := range regions {
		go func() {
			duration, err := probe(ctx, region)
			if err != nil {
				duration = 0
			}
			results <- measurement{region, duration}
		}()
	}
	latencies := make(map[string]time.Duration, len(zones))
	for range regions {
		result := <-results
		if result.duration > 0 {
			for _, zone := range regions[result.region] {
				latencies[zone] = result.duration
			}
		}
	}
	if len(latencies) == 0 {
		return nil, errors.New(locale.T("Could not measure latency to Google Cloud. Choose a zone from the list."))
	}
	return latencies, nil
}

func probeRegion(ctx context.Context, region string) (time.Duration, error) {
	// Cloud Storage regional endpoints terminate TLS in the specified region.
	// https://docs.cloud.google.com/storage/docs/regional-endpoints
	endpoint := net.JoinHostPort("storage."+region+".rep.googleapis.com", "443")
	started := time.Now()
	conn, err := (&tls.Dialer{}).DialContext(ctx, "tcp", endpoint)
	if err != nil {
		return 0, err
	}
	duration := time.Since(started)
	_ = conn.Close()
	return duration, nil
}
