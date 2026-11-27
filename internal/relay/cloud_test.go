package relay

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func reply(code int, body string) (*http.Response, error) {
	return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body))}, nil
}

func TestCreateRetriesWithOneVMNameAndRequestID(t *testing.T) {
	config := Config{Project: "test-project", Zone: "southamerica-east1-a", Network: "default", Domain: "mc.example.com"}
	session := identity{RequestID: requestID(), Nonce: "same-session", Deadline: time.Now().UTC().Truncate(time.Second).Add(4 * time.Hour)}
	calls := 0
	client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Method != http.MethodPost || r.URL.Path != "/compute/v1/projects/test-project/zones/southamerica-east1-a/instances" ||
			r.URL.Query().Get("requestId") != session.RequestID {
			t.Fatalf("unexpected create request: %s %s", r.Method, r.URL)
		}
		var vm struct {
			Name        string
			MachineType string
			Labels      map[string]string
			Scheduling  map[string]string
			Disks       []struct {
				AutoDelete       bool
				InitializeParams struct{ DiskType string }
			}
		}
		if err := json.NewDecoder(r.Body).Decode(&vm); err != nil {
			t.Fatal(err)
		}
		if vm.Name != config.name() || vm.Labels["session"] != session.Nonce ||
			vm.Scheduling["instanceTerminationAction"] != "DELETE" ||
			vm.Scheduling["terminationTime"] != session.Deadline.Format(time.RFC3339) ||
			vm.MachineType != "zones/southamerica-east1-a/machineTypes/e2-micro" ||
			len(vm.Disks) != 1 || !vm.Disks[0].AutoDelete ||
			vm.Disks[0].InitializeParams.DiskType != "zones/southamerica-east1-a/diskTypes/pd-standard" {
			t.Fatalf("incorrect VM identity or cleanup configuration: %+v", vm)
		}
		if calls == 1 {
			return nil, errors.New("response lost")
		}
		return reply(200, `{"status":"DONE"}`)
	})}
	if err := (compute{client, config}).create(t.Context(), session); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("sent %d requests, want 2", calls)
	}
}

func TestCheckMachineTypeUsesSelectedZone(t *testing.T) {
	for name, status := range map[string]int{
		"available":    http.StatusOK,
		"missing":      http.StatusNotFound,
		"unauthorized": http.StatusForbidden,
	} {
		t.Run(name, func(t *testing.T) {
			config := Config{Project: "test-project", Zone: "europe-west2-a"}
			client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method != http.MethodGet || r.URL.Path != "/compute/v1/projects/test-project/zones/europe-west2-a/machineTypes/e2-micro" {
					t.Fatalf("unexpected machine type request: %s %s", r.Method, r.URL)
				}
				return reply(status, `{}`)
			})}
			err := (compute{client, config}).checkMachineType(t.Context())
			if status == http.StatusOK {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), config.Zone) {
				t.Fatalf("machine type check error = %v", err)
			}
		})
	}
}

func TestExtendChangesOnlyTheOwnedVMDeadline(t *testing.T) {
	config := Config{Project: "test-project", Zone: "asia-northeast1-a", Domain: "mc.example.com"}
	created := time.Now().UTC().Truncate(time.Second)
	before := created.Add(4 * time.Hour)
	for _, owner := range []bool{true, false} {
		if owner {
			t.Run("owner", func(t *testing.T) { testExtension(t, config, created, before, "local") })
		} else {
			t.Run("other session", func(t *testing.T) { testExtension(t, config, created, before, "another") })
		}
	}
}

func testExtension(t *testing.T, config Config, created, before time.Time, nonce string) {
	t.Helper()
	updates := 0
	client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/instances/"+config.name()):
			vm := instance{
				ID: "42", Status: "RUNNING", CreationTimestamp: created,
				Labels: map[string]string{"managed-by": "tovela", "relay": config.name(), "session": nonce},
			}
			vm.Scheduling.TerminationTime = before
			data, err := json.Marshal(vm)
			if err != nil {
				t.Fatal(err)
			}
			return reply(200, string(data))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/setScheduling"):
			updates++
			var scheduling map[string]string
			if err := json.NewDecoder(r.Body).Decode(&scheduling); err != nil {
				t.Fatal(err)
			}
			if scheduling["terminationTime"] != before.Add(4*time.Hour).Format(time.RFC3339) ||
				scheduling["instanceTerminationAction"] != "DELETE" || r.URL.Query().Get("requestId") == "" {
				t.Fatalf("unexpected schedule: %+v", scheduling)
			}
			return reply(200, `{"status":"DONE"}`)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL)
			return nil, nil
		}
	})}
	got, err := (compute{client, config}).extend(t.Context(), identity{Nonce: "local", InstanceID: "42"})
	if nonce != "local" {
		if err == nil || updates != 0 {
			t.Fatalf("attempted to extend a different session: %v, %d updates", err, updates)
		}
		return
	}
	if err != nil || updates != 1 || !got.Equal(before.Add(4*time.Hour)) {
		t.Fatalf("extend = %s, %v, %d updates", got, err, updates)
	}
}

func TestExistingDNSRequiresReplacementConsent(t *testing.T) {
	config := DefaultConfig("", "")
	config.Domain = "mc.example.com"
	dns := cloudflare{config: config, client: &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("name") == config.Domain {
			return reply(200, `{"result":[{"id":"other","type":"CNAME","name":"mc.example.com"}]}`)
		}
		return reply(200, `{"result":[{"id":"srv","type":"SRV","name":"_minecraft._tcp.mc.example.com"}]}`)
	})}}
	const root = "https://example.com/dns_records"
	var conflict *DNSConflict
	if _, err := dns.ensureAvailable(t.Context(), root, identity{Nonce: "local"}, false); !errors.As(err, &conflict) || conflict.Hostname != config.Domain {
		t.Fatalf("expected a DNS conflict for %s, got %v", config.Domain, err)
	}
	deletes, err := dns.ensureAvailable(t.Context(), root, identity{Nonce: "local"}, true)
	if err != nil || len(deletes) != 2 || deletes[0]["id"] != "other" || deletes[1]["id"] != "srv" {
		t.Fatalf("unexpected replacement plan: %+v, %v", deletes, err)
	}
}

func TestCloudflareZoneSelection(t *testing.T) {
	for _, tc := range []struct {
		name     string
		override string
		zones    map[string]string
		want     string
	}{
		{name: "nearest managed zone", zones: map[string]string{"games.example.com": "games", "example.com": "root"}, want: "games"},
		{name: "parent zone", zones: map[string]string{"example.com": "root"}, want: "root"},
		{name: "explicit zone", override: "example.com", zones: map[string]string{"games.example.com": "games", "example.com": "root"}, want: "root"},
		{name: "missing explicit zone", override: "games.example.com", zones: map[string]string{"example.com": "root"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				if id := tc.zones[r.URL.Query().Get("name")]; id != "" {
					return reply(200, `{"result":[{"id":"`+id+`"}]}`)
				}
				return reply(200, `{"result":[]}`)
			})}
			dns := cloudflare{client, Config{Domain: "play.games.example.com", CloudflareZone: tc.override}}
			id, err := dns.zone(t.Context())
			if id != tc.want || (err != nil) != (tc.want == "") {
				t.Fatalf("zone() = %q, %v; want %q", id, err, tc.want)
			}
		})
	}
}
