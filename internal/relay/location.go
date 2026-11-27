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

// RelayZones lists zones where the selected project can see the relay machine type.
func (d *Desktop) RelayZones(ctx context.Context, project, googleID string) ([]string, error) {
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

func (c compute) relayZones(ctx context.Context) ([]string, error) {
	query := url.Values{
		"filter":               {fmt.Sprintf("name = %q", relayMachineType)},
		"returnPartialSuccess": {"true"},
	}
	found := make(map[string]struct{})
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
				if machine.Name == relayMachineType {
					found[zone] = struct{}{}
					break
				}
			}
		}
		if page.NextPageToken == "" {
			break
		}
		query.Set("pageToken", page.NextPageToken)
	}
	zones := make([]string, 0, len(found))
	for zone := range found {
		zones = append(zones, zone)
	}
	slices.Sort(zones)
	return zones, nil
}

// FastestRelayZone estimates latency from this computer using regional Google endpoints.
func (d *Desktop) FastestRelayZone(ctx context.Context, zones []string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	return fastestRelayZone(ctx, zones, probeRegion)
}

func fastestRelayZone(ctx context.Context, zones []string, probe func(context.Context, string) (time.Duration, error)) (string, error) {
	candidates := make(map[string]string)
	for _, zone := range zones {
		if !validZone(zone) {
			continue
		}
		region := zone[:len(zone)-2]
		if previous := candidates[region]; previous == "" || zone < previous {
			candidates[region] = zone
		}
	}
	type measurement struct {
		zone     string
		duration time.Duration
		err      error
	}
	results := make(chan measurement, len(candidates))
	for region, zone := range candidates {
		go func() {
			duration, err := probe(ctx, region)
			results <- measurement{zone, duration, err}
		}()
	}
	var best measurement
	for range candidates {
		result := <-results
		if result.err == nil && (best.zone == "" || result.duration < best.duration ||
			(result.duration == best.duration && result.zone < best.zone)) {
			best = result
		}
	}
	if best.zone == "" {
		return "", errors.New(locale.T("Could not measure latency to Google Cloud. Choose a zone from the list."))
	}
	return best.zone, nil
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
