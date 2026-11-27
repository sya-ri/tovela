package relay

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestNewProfileDoesNotInheritLocation(t *testing.T) {
	current := DefaultConfig("", "")
	current.Project, current.Zone, current.Network = "test-project", "europe-west2-a", "relay-vpc"
	current.Ports = []PortMapping{{TCP, 8080, 80}, {UDP, 19132, 19132}}
	desktop := &Desktop{
		root:   t.TempDir(),
		active: "default",
		profiles: map[string]*Service{
			"default": {state: State{Config: current, Phase: "idle"}},
		},
	}
	if err := desktop.AddProfile(); err != nil {
		t.Fatal(err)
	}
	got := desktop.GetState().Config
	if got.Project != current.Project || got.Network != current.Network || got.Zone != "" {
		t.Errorf("new profile location configuration = %s/%s on %s", got.Project, got.Zone, got.Network)
	}
	if !slices.Equal(got.Ports, []PortMapping{{MinecraftJava, 25565, 25565}}) {
		t.Errorf("new profile received non-default ports or protocol: %+v", got)
	}
}

func TestConfigRelayZone(t *testing.T) {
	config := DefaultConfig("", "")
	config.Project, config.Domain = "test-project", "mc.example.com"
	if config.Zone != "" {
		t.Fatalf("new relay has an assigned zone: %q", config.Zone)
	}
	if err := config.validate(); err != nil {
		t.Fatalf("a draft with no zone should be valid: %v", err)
	}
	for _, tc := range []struct {
		zone  string
		valid bool
	}{
		{"asia-northeast1-b", true},
		{"europe-west10-a", true},
		{"northamerica-northeast2-c", true},
		{" southamerica-east1-a ", true},
		{"africa-south1-a", true},
		{"", false},
		{"europe-west2", false},
		{"europe-west2-a/instances", false},
	} {
		t.Run(tc.zone, func(t *testing.T) {
			config.Zone = tc.zone
			config.normalize()
			if err := config.validateRelay(); (err == nil) != tc.valid {
				t.Errorf("validateRelay() = %v for zone %q", err, tc.zone)
			}
		})
	}
}

func TestLoadConfigMigratesLegacyPorts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"TCPPort":25570,"UDPPort":19132}`), 0600); err != nil {
		t.Fatal(err)
	}
	config, err := loadConfig(path, DefaultConfig("", ""))
	want := []PortMapping{{MinecraftJava, 25570, 25565}, {UDP, 19132, 19132}}
	if config.RelayMachineType() != DefaultMachineType {
		t.Errorf("legacy config machine type = %q, want %q", config.RelayMachineType(), DefaultMachineType)
	}
	if err != nil || !slices.Equal(config.Ports, want) {
		t.Fatalf("migrated mappings = %+v, %v, want %+v", config.Ports, err, want)
	}
}
