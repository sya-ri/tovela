package relay

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sya-ri/tovela/internal/locale"
	"golang.org/x/oauth2"
)

const credentialService = "tovela"

const DefaultTCPPort = 25565
const DefaultMachineType = "e2-micro"

var machineTypes = [...]struct {
	name      string
	memoryMiB int
}{
	{DefaultMachineType, 1024},
	{"e2-small", 2048},
	{"e2-medium", 4096},
	{"e2-standard-2", 8192},
}

func MachineTypes() []string {
	names := make([]string, 0, len(machineTypes))
	for _, machine := range machineTypes {
		names = append(names, machine.name)
	}
	return names
}

var (
	projectPattern  = regexp.MustCompile("^[a-z][a-z0-9-]{4,28}[a-z0-9]$")
	zonePattern     = regexp.MustCompile("^(?:[a-z][a-z0-9]*-)+[a-z][a-z0-9]*[0-9]-[a-z]$")
	resourcePattern = regexp.MustCompile("^[a-z]([-a-z0-9]{0,61}[a-z0-9])?$")
	domainPattern   = regexp.MustCompile("^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\\.)+[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$")
)

type Config struct {
	MachineType        string
	Project            string
	Zone               string
	Network            string
	Domain             string
	CloudflareZone     string
	Ports              []PortMapping
	Hours              int
	GoogleClientID     string
	CloudflareClientID string
}

func DefaultConfig(googleID, cloudflareID string) Config {
	return Config{
		MachineType: DefaultMachineType, Network: "default", Hours: 4, GoogleClientID: googleID, CloudflareClientID: cloudflareID,
		Ports: (legacyPorts{}).mappings(),
	}
}

func (c Config) RelayMachineType() string {
	if c.MachineType == "" {
		return DefaultMachineType
	}
	return c.MachineType
}

func (c Config) RelayMemoryMiB() int {
	for _, machine := range machineTypes {
		if machine.name == c.RelayMachineType() {
			return machine.memoryMiB
		}
	}
	return 0
}

func (c *Config) normalize() {
	c.MachineType = c.RelayMachineType()
	c.Domain = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(c.Domain)), ".")
	c.CloudflareZone = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(c.CloudflareZone)), ".")
	c.Project = strings.TrimSpace(c.Project)
	c.Zone = strings.ToLower(strings.TrimSpace(c.Zone))
	c.GoogleClientID = strings.TrimSpace(c.GoogleClientID)
	c.CloudflareClientID = strings.TrimSpace(c.CloudflareClientID)
}

func (c Config) validate() error {
	if c.RelayMemoryMiB() == 0 {
		return errors.New(locale.T("Choose a supported machine type."))
	}
	if !projectPattern.MatchString(c.Project) || !resourcePattern.MatchString(c.Network) {
		return errors.New(locale.T("Enter a valid Google Cloud project ID and VPC network name."))
	}
	if c.Zone != "" && !validZone(c.Zone) {
		return errors.New(locale.T("Choose a valid Google Cloud zone."))
	}
	if len(c.Domain) > 253 || !domainPattern.MatchString(c.Domain) {
		return errors.New(locale.T("Enter a valid hostname."))
	}
	if c.CloudflareZone != "" && (!domainPattern.MatchString(c.CloudflareZone) ||
		(c.Domain != c.CloudflareZone && !strings.HasSuffix(c.Domain, "."+c.CloudflareZone))) {
		return errors.New(locale.T("Enter a Cloudflare zone that contains the hostname."))
	}
	if c.Hours < 1 || c.Hours > 12 {
		return errors.New(locale.T("Enter an auto-delete interval from 1 to 12 hours."))
	}
	return validatePortMappings(c.Ports)
}

func (c Config) validateRelay() error {
	if c.Zone == "" {
		return errors.New(locale.T("Choose a relay zone."))
	}
	if len(c.Ports) == 0 {
		return errors.New(locale.T("Add a port mapping before starting the relay."))
	}
	return c.validate()
}

func (c Config) name() string {
	sum := sha256.Sum256([]byte(c.Domain))
	return fmt.Sprintf("tovela-%x", sum[:10])
}

func (c Config) sessionKey() string {
	return strings.Join([]string{"relay", c.Project, c.Zone, c.name()}, "/")
}

func loadConfig(path string, defaults Config) (Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return defaults, nil
	}
	if err != nil {
		return defaults, err
	}
	loaded := struct {
		Config
		legacyPorts
	}{Config: defaults}
	loaded.Ports = nil
	if err := json.Unmarshal(data, &loaded); err != nil {
		return defaults, err
	}
	if loaded.Ports == nil {
		loaded.Ports = loaded.legacyPorts.mappings()
	}
	return loaded.Config, nil
}

func saveConfig(path string, config Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), "settings-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	_, writeErr := temp.Write(data)
	if err := errors.Join(writeErr, temp.Close()); err != nil {
		return err
	}
	return os.Rename(temp.Name(), path)
}

func (c Config) oauth(provider string) (*oauth2.Config, string, error) {
	cfg := &oauth2.Config{}
	switch provider {
	case "google":
		cfg.ClientID = c.GoogleClientID
		cfg.Endpoint = oauth2.Endpoint{AuthURL: "https://accounts.google.com/o/oauth2/v2/auth", TokenURL: "https://oauth2.googleapis.com/token", AuthStyle: oauth2.AuthStyleInParams}
		cfg.Scopes = []string{"https://www.googleapis.com/auth/compute", "https://www.googleapis.com/auth/monitoring.read"}
	case "cloudflare":
		cfg.ClientID = c.CloudflareClientID
		cfg.Endpoint = oauth2.Endpoint{AuthURL: "https://dash.cloudflare.com/oauth2/auth", TokenURL: "https://dash.cloudflare.com/oauth2/token", AuthStyle: oauth2.AuthStyleInParams}
		cfg.Scopes = []string{"zone.read", "dns.write", "offline_access"}
	default:
		return nil, "", errors.New(locale.T("unsupported sign-in provider"))
	}
	if cfg.ClientID == "" {
		return nil, "", fmt.Errorf(locale.T("Enter the %s OAuth client ID."), provider)
	}
	key := sha256.Sum256([]byte(cfg.ClientID))
	return cfg, fmt.Sprintf("%s-%x", provider, key[:10]), nil
}
