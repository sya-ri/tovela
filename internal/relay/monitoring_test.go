package relay

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestReadRelayLoadSelectsLatestMetricsForInstance(t *testing.T) {
	now := time.Date(2026, 11, 28, 9, 25, 0, 0, time.UTC)
	config := Config{Project: "example-project", Zone: "europe-west2-a"}
	client := &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
		query := req.URL.Query()
		filter := query.Get("filter")
		if req.Method != http.MethodGet || req.URL.Host != "monitoring.googleapis.com" ||
			req.URL.Path != "/v3/projects/example-project/timeSeries" ||
			!strings.Contains(filter, `resource.type = "gce_instance" AND resource.labels.instance_id = "18446744073709551615" AND resource.labels.zone = "europe-west2-a" AND resource.labels.project_id = "example-project"`) ||
			query.Get("interval.startTime") != "2026-11-28T09:15:00Z" ||
			query.Get("interval.endTime") != "2026-11-28T09:25:00Z" ||
			query.Get("view") != "FULL" {
			return nil, fmt.Errorf("unexpected monitoring request: %s %s", req.Method, req.URL)
		}
		switch {
		case strings.Contains(filter, cpuUtilization):
			return reply(200, `{"timeSeries":[{"points":[
				{"interval":{"endTime":"2026-11-28T09:20:00Z"},"value":{"doubleValue":0.4}},
				{"interval":{"endTime":"2026-11-28T09:23:00Z"},"value":{"doubleValue":1.2}}
			]}]}`)
		case strings.Contains(filter, memoryUsed):
			return reply(200, `{"timeSeries":[{"points":[
				{"interval":{"endTime":"2026-11-28T09:22:00Z"},"value":{"int64Value":"0"}}
			]}]}`)
		default:
			return nil, errors.New("unexpected metric")
		}
	})}
	got, err := readRelayLoad(t.Context(), client, config, "18446744073709551615", now)
	if err != nil || got.CPUFraction == nil || *got.CPUFraction != 1.2 || !got.CPUTime.Equal(now.Add(-2*time.Minute)) ||
		got.MemoryBytes == nil || *got.MemoryBytes != 0 || !got.MemoryTime.Equal(now.Add(-3*time.Minute)) {
		t.Fatalf("load = %+v, error = %v; want the newest CPU and memory samples", got, err)
	}
}

func TestReadRelayLoadDistinguishesMissingDataAndPermissions(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusForbidden} {
		client := &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
			return reply(status, `{}`)
		})}
		load, err := readRelayLoad(t.Context(), client, Config{Project: "example-project", Zone: "us-central1-a"}, "123", time.Now())
		if load.CPUFraction != nil || load.MemoryBytes != nil || (err != nil) != (status == http.StatusForbidden) {
			t.Errorf("status %d: load = %+v, error = %v", status, load, err)
		}
	}
}
