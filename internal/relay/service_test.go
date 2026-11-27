package relay

import (
	"errors"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/zalando/go-keyring"
	"golang.org/x/oauth2"
)

func TestStartChecksDNSBeforePreparingClient(t *testing.T) {
	keyring.MockInit()
	t.Cleanup(keyring.MockInit)
	config := DefaultConfig("google-client", "cloudflare-client")
	config.Project, config.Zone, config.Domain = "test-project", "us-central1-a", "play.example.com"
	config.Ports = []PortMapping{{UDP, 19132, 19132}}
	token := oauth2.Token{AccessToken: "saved", RefreshToken: "refresh", Expiry: time.Now().Add(time.Hour)}
	for _, provider := range []string{"google", "cloudflare"} {
		_, key, err := config.oauth(provider)
		if err != nil {
			t.Fatal(err)
		}
		if err := writeCredential(key, token); err != nil {
			t.Fatal(err)
		}
	}
	previous := http.DefaultTransport
	http.DefaultTransport = transportFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("unexpected write during preflight: %s %s", req.Method, req.URL)
		}
		switch req.URL.Host + req.URL.Path {
		case "compute.googleapis.com/compute/v1/projects/test-project/zones/us-central1-a/instances/" + config.name():
			return reply(http.StatusNotFound, "{}")
		case "api.cloudflare.com/client/v4/zones":
			return reply(http.StatusOK, `{"result":[{"id":"test-zone"}]}`)
		case "api.cloudflare.com/client/v4/zones/test-zone/dns_records":
			return reply(http.StatusOK, `{"result":[{"id":"existing","type":"A","name":"play.example.com"}]}`)
		default:
			t.Fatalf("unexpected preflight request: %s", req.URL)
			return nil, nil
		}
	})
	t.Cleanup(func() { http.DefaultTransport = previous })

	// Client extraction would fail here even if a bundled executable is present.
	missingTemp := filepath.Join(t.TempDir(), "missing")
	for _, variable := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(variable, missingTemp)
	}
	service := &Service{state: State{Config: config, Phase: "idle"}}
	var conflict *DNSConflict
	if err := service.Start(t.Context(), false); !errors.As(err, &conflict) || conflict.Hostname != config.Domain {
		t.Fatalf("Start() = %v, want the DNS conflict for %s", err, config.Domain)
	}
}
