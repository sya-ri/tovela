package relay

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"sync"
	"time"

	"github.com/sya-ri/tovela/internal/locale"
)

const LoadWindow = 30 * time.Minute

const (
	cpuUtilization = "compute.googleapis.com/instance/cpu/utilization"
	memoryUsed     = "compute.googleapis.com/instance/memory/balloon/ram_used"
)

type Load struct {
	End        time.Time
	CPUPercent []LoadSample
	MemoryMiB  []LoadSample
}

type LoadSample struct {
	At    time.Time
	Value float64
}

type monitoringPoint struct {
	Interval struct{ EndTime time.Time }
	Value    struct {
		DoubleValue *float64
		Int64Value  *int64 `json:"int64Value,string"`
	}
}

// RelayLoad reads recent host metrics for the instance owned by the running profile.
func (d *Desktop) RelayLoad(ctx context.Context, profileID string) (Load, error) {
	d.mu.Lock()
	profile := d.profiles[profileID]
	d.mu.Unlock()
	if profile == nil {
		return Load{}, errors.New(locale.T("The relay is no longer running."))
	}
	state := profile.GetState()
	if state.Phase != "running" {
		return Load{}, errors.New(locale.T("The relay is no longer running."))
	}
	var session identity
	if err := readCredential(state.Config.sessionKey(), &session); err != nil || session.InstanceID == "" {
		return Load{}, errors.New(locale.T("Could not identify the running relay."))
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	client, err := authenticatedClient(ctx, state.Config, "google")
	if err != nil {
		return Load{}, err
	}
	return readRelayLoad(ctx, client, state.Config, session.InstanceID, time.Now())
}

func readRelayLoad(ctx context.Context, client *http.Client, config Config, instanceID string, now time.Time) (Load, error) {
	load := Load{End: now.Truncate(time.Second)}
	var cpuErr, memoryErr error
	var requests sync.WaitGroup
	requests.Go(func() {
		load.CPUPercent, cpuErr = metricHistory(ctx, client, config, instanceID, cpuUtilization, load.End)
	})
	requests.Go(func() {
		load.MemoryMiB, memoryErr = metricHistory(ctx, client, config, instanceID, memoryUsed, load.End)
	})
	requests.Wait()
	if isHTTP(cpuErr, http.StatusForbidden) || isHTTP(memoryErr, http.StatusForbidden) {
		return load, errors.New(locale.T("Cloud Monitoring access denied. Check permissions and sign in to Google again."))
	}
	if err := errors.Join(cpuErr, memoryErr); err != nil {
		return load, fmt.Errorf(locale.T("Could not read relay load: %w"), err)
	}
	return load, nil
}

func metricHistory(ctx context.Context, client *http.Client, config Config, instanceID, metric string, now time.Time) ([]LoadSample, error) {
	start := now.Add(-LoadWindow)
	query := url.Values{
		"filter":             {fmt.Sprintf(`metric.type = %q AND resource.type = "gce_instance" AND resource.labels.instance_id = %q AND resource.labels.zone = %q AND resource.labels.project_id = %q`, metric, instanceID, config.Zone, config.Project)},
		"interval.startTime": {start.UTC().Format(time.RFC3339)},
		"interval.endTime":   {now.UTC().Format(time.RFC3339)},
		"view":               {"FULL"},
	}
	var response struct {
		TimeSeries []struct{ Points []monitoringPoint }
	}
	address := "https://monitoring.googleapis.com/v3/projects/" + config.Project + "/timeSeries?" + query.Encode()
	if err := request(ctx, client, http.MethodGet, address, nil, &response); err != nil {
		return nil, err
	}
	var samples []LoadSample
	for _, series := range response.TimeSeries {
		for _, point := range series.Points {
			at := point.Interval.EndTime
			if at.Before(start) || at.After(now) {
				continue
			}
			switch {
			case metric == cpuUtilization && point.Value.DoubleValue != nil && *point.Value.DoubleValue >= 0:
				samples = append(samples, LoadSample{At: at, Value: *point.Value.DoubleValue * 100})
			case metric == memoryUsed && point.Value.Int64Value != nil && *point.Value.Int64Value >= 0:
				samples = append(samples, LoadSample{At: at, Value: float64(*point.Value.Int64Value) / (1 << 20)})
			}
		}
	}
	slices.SortFunc(samples, func(a, b LoadSample) int { return a.At.Compare(b.At) })
	return slices.CompactFunc(samples, func(a, b LoadSample) bool { return a.At.Equal(b.At) }), nil
}
