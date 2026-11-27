package relay

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/sya-ri/tovela/internal/locale"
)

const (
	cpuUtilization = "compute.googleapis.com/instance/cpu/utilization"
	memoryUsed     = "compute.googleapis.com/instance/memory/balloon/ram_used"
)

type Load struct {
	CPUFraction *float64
	CPUTime     time.Time
	MemoryBytes *int64
	MemoryTime  time.Time
}

type monitoringPoint struct {
	Interval struct{ EndTime time.Time }
	Value    struct {
		DoubleValue *float64
		Int64Value  *int64 `json:"int64Value,string"`
	}
}

// RelayLoad reads the latest host metrics for the instance owned by the running profile.
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
	var (
		load              Load
		cpuErr, memoryErr error
		requests          sync.WaitGroup
	)
	requests.Go(func() {
		point, err := latestMetric(ctx, client, config, instanceID, cpuUtilization, now)
		load.CPUFraction, load.CPUTime, cpuErr = point.Value.DoubleValue, point.Interval.EndTime, err
	})
	requests.Go(func() {
		point, err := latestMetric(ctx, client, config, instanceID, memoryUsed, now)
		load.MemoryBytes, load.MemoryTime, memoryErr = point.Value.Int64Value, point.Interval.EndTime, err
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

func latestMetric(ctx context.Context, client *http.Client, config Config, instanceID, metric string, now time.Time) (monitoringPoint, error) {
	query := url.Values{
		"filter":             {fmt.Sprintf(`metric.type = %q AND resource.type = "gce_instance" AND resource.labels.instance_id = %q AND resource.labels.zone = %q AND resource.labels.project_id = %q`, metric, instanceID, config.Zone, config.Project)},
		"interval.startTime": {now.Add(-10 * time.Minute).UTC().Format(time.RFC3339)},
		"interval.endTime":   {now.UTC().Format(time.RFC3339)},
		"view":               {"FULL"},
	}
	var response struct {
		TimeSeries []struct{ Points []monitoringPoint }
	}
	address := "https://monitoring.googleapis.com/v3/projects/" + config.Project + "/timeSeries?" + query.Encode()
	if err := request(ctx, client, http.MethodGet, address, nil, &response); err != nil {
		return monitoringPoint{}, err
	}
	var latest monitoringPoint
	for _, series := range response.TimeSeries {
		for _, point := range series.Points {
			hasValue := point.Value.DoubleValue != nil && metric == cpuUtilization ||
				point.Value.Int64Value != nil && metric == memoryUsed
			if hasValue && point.Interval.EndTime.After(latest.Interval.EndTime) {
				latest = point
			}
		}
	}
	return latest, nil
}
