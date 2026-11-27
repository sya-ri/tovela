package relay

import "testing"

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
