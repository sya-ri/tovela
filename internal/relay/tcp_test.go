package relay

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestRawTCPUsesPublicPortAndAcceptsAnHTTPService(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	address, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	host, portText, err := net.SplitHostPort(address.Host)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	config := DefaultConfig("", "")
	web := PortMapping{TCP, port, 80}
	config.Ports = append(config.Ports, web, PortMapping{UDP, 19132, 19132}, PortMapping{UDP, 8080, 80})
	config.Project, config.Domain = "example-project", "web.example.com"
	if err := config.validate(); err != nil {
		t.Fatal(err)
	}
	if err := web.probeTCP(t.Context(), host, config.Domain, port); err != nil {
		t.Fatalf("could not probe the local HTTP service over TCP: %v", err)
	}
	session := identity{Ports: config.Ports}
	client := frpcConfig(config, session, "192.0.2.1", "server.crt")
	if !strings.Contains(client, "localPort = "+portText+"\nremotePort = 80") ||
		strings.Count(client, "[[proxies]]") != 4 ||
		!strings.Contains(serverStartup(session), "allowPorts = [{single = 25565}, {single = 80}, {single = 19132}]") {
		t.Fatal("the relay configuration did not use all public TCP and UDP ports")
	}
	config.Ports = append(config.Ports, PortMapping{MinecraftJava, 25566, 80})
	if err := config.validate(); err == nil {
		t.Fatal("accepted duplicate public TCP port 80")
	}
}

func TestWaitReadyRequiresAllUDPMappings(t *testing.T) {
	config := Config{Domain: "play.example.com", Ports: []PortMapping{
		{UDP, 24454, 24454},
		{UDP, 19132, 19132},
	}}
	client := &tunnel{done: make(chan struct{}), udpReady: make(chan string, 2)}
	client.udpReady <- config.Ports[0].proxyName()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := client.waitReady(ctx, config, "127.0.0.1")
	if err == nil || !strings.Contains(err.Error(), "19132") {
		t.Fatalf("expected the second UDP mapping to remain pending, got %v", err)
	}

	client.udpReady <- config.Ports[0].proxyName()
	client.udpReady <- config.Ports[1].proxyName()
	if err := client.waitReady(t.Context(), config, "127.0.0.1"); err != nil {
		t.Fatalf("all UDP mappings should be ready: %v", err)
	}
}

func TestDecodeLegacySessionPorts(t *testing.T) {
	var session identity
	if err := json.Unmarshal([]byte(`{"RequestID":"old","TCPPort":25570,"UDPPort":19132}`), &session); err != nil {
		t.Fatal(err)
	}
	want := []PortMapping{{MinecraftJava, 25570, 25565}, {UDP, 19132, 19132}}
	if session.RequestID != "old" || !slices.Equal(session.mappings(), want) {
		t.Fatalf("legacy session decoded as %+v, want %+v", session, want)
	}
}
