package relay

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/sya-ri/tovela/internal/locale"
)

type Profile struct {
	ID     string
	Domain string
	Phase  string
}

type DesktopState struct {
	State
	ActiveProfile string
	Profiles      []Profile
}

// Desktop exposes the selected profile to the window while each relay manages its own lifetime.
type Desktop struct {
	mu       sync.Mutex
	root     string
	active   string
	defaults Config
	profiles map[string]*Service
}

func NewDesktop(googleID, cloudflareID string) (*Desktop, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	return openDesktop(filepath.Join(dir, "tovela", "profiles"), DefaultConfig(googleID, cloudflareID))
}

func openDesktop(root string, defaults Config) (*Desktop, error) {
	desktop := &Desktop{root: root, defaults: defaults, profiles: make(map[string]*Service)}
	dirs, err := os.ReadDir(root)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	for _, entry := range dirs {
		if !entry.IsDir() {
			continue
		}
		path := desktop.path(entry.Name())
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return nil, err
		}
		profile, err := newService(path, desktop.defaults)
		if err != nil {
			return nil, err
		}
		desktop.profiles[entry.Name()] = profile
		if desktop.active == "" {
			desktop.active = entry.Name()
		}
	}
	if desktop.active == "" {
		profile, err := newService(desktop.path("default"), desktop.defaults)
		if err != nil {
			return nil, err
		}
		desktop.active = "default"
		desktop.profiles[desktop.active] = profile
	}
	return desktop, nil
}

func (d *Desktop) path(id string) string {
	return filepath.Join(d.root, id, "settings.json")
}

func (d *Desktop) GetState() DesktopState {
	d.mu.Lock()
	defer d.mu.Unlock()
	profiles := make([]Profile, 0, len(d.profiles))
	for id, service := range d.profiles {
		state := service.GetState()
		profiles = append(profiles, Profile{ID: id, Domain: state.Config.Domain, Phase: state.Phase})
	}
	sort.Slice(profiles, func(i, j int) bool {
		if profiles[i].Domain != profiles[j].Domain {
			return profiles[i].Domain < profiles[j].Domain
		}
		return profiles[i].ID < profiles[j].ID
	})
	return DesktopState{State: d.profiles[d.active].GetState(), ActiveProfile: d.active, Profiles: profiles}
}

func (d *Desktop) SelectProfile(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.profiles[id] == nil {
		return errors.New(locale.T("This relay profile does not exist."))
	}
	d.active = id
	return nil
}

func (d *Desktop) AddProfile() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	current := d.profiles[d.active].GetState().Config
	config := DefaultConfig(current.GoogleClientID, current.CloudflareClientID)
	config.Project, config.Network = current.Project, current.Network
	id := requestID()
	path := d.path(id)
	if err := saveConfig(path, config); err != nil {
		return err
	}
	profile, err := newService(path, config)
	if err != nil {
		return err
	}
	d.profiles[id], d.active = profile, id
	return nil
}

// DeleteProfile removes an idle profile after its relay has been stopped.
func (d *Desktop) DeleteProfile(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	profile := d.profiles[id]
	if profile == nil {
		return errors.New(locale.T("This relay profile does not exist."))
	}
	if !profile.operation.TryLock() {
		return errors.New(locale.T("An operation is already in progress."))
	}
	defer profile.operation.Unlock()
	if profile.GetState().Phase != "idle" {
		return errors.New(locale.T("Stop the relay before deleting this profile."))
	}
	if err := os.Remove(profile.configPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	// Other folders or files in the profile directory are left alone.
	_ = os.Remove(filepath.Dir(profile.configPath))
	delete(d.profiles, id)
	if len(d.profiles) == 0 {
		replacement := &Service{configPath: d.path("default"), state: State{Config: d.defaults, Phase: "idle"}}
		replacement.refreshLogins()
		d.profiles["default"], d.active = replacement, "default"
	} else if d.active == id {
		d.active = ""
		for next := range d.profiles {
			if d.active == "" || next < d.active {
				d.active = next
			}
		}
	}
	return nil
}

func (d *Desktop) SaveConfig(config Config) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	config.normalize()
	for id, service := range d.profiles {
		if id != d.active && config.Domain != "" && service.GetState().Config.Domain == config.Domain {
			return errors.New(locale.T("A relay profile already uses this hostname."))
		}
	}
	return d.profiles[d.active].SaveConfig(config)
}

func (d *Desktop) selected() *Service {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.profiles[d.active]
}

func (d *Desktop) LogIn(ctx context.Context, provider string) error {
	if err := d.selected().LogIn(ctx, provider); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, service := range d.profiles {
		service.refreshLogins()
	}
	return nil
}

func (d *Desktop) Start(ctx context.Context, replaceDNS bool) error {
	return d.selected().Start(ctx, replaceDNS)
}

func (d *Desktop) Extend(ctx context.Context) error {
	return d.selected().Extend(ctx)
}

func (d *Desktop) Stop(ctx context.Context) error {
	return d.selected().Stop(ctx)
}

func (d *Desktop) ServiceShutdown() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	results := make(chan error, len(d.profiles))
	for _, profile := range d.profiles {
		go func() { results <- profile.closeApp() }()
	}
	var err error
	for range d.profiles {
		err = errors.Join(err, <-results)
	}
	return err
}
