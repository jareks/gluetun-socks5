package socks5

import (
	"context"
	"sync"
	"reflect"

	"github.com/qdm12/gluetun/internal/configuration/settings"
	"github.com/qdm12/gluetun/internal/models"
	"github.com/qdm12/gluetun/internal/constants"
)

func NewState(statusApplier StatusApplier,
	settings settings.Socks5,
) *State {
	return &State{
		statusApplier: statusApplier,
		settings:      settings,
	}
}

type State struct {
	statusApplier StatusApplier
	settings      settings.Socks5
	settingsMu    sync.RWMutex
}

type StatusApplier interface {
	ApplyStatus(ctx context.Context, status models.LoopStatus) (
		outcome string, err error)
}

func (s *State) GetSettings() (settings settings.Socks5) {
	s.settingsMu.RLock()
	defer s.settingsMu.RUnlock()
	return s.settings
}

func (s *State) SetSettings(ctx context.Context,
	settings settings.Socks5,
) (outcome string) {
	s.settingsMu.Lock()
	settingsUnchanged := reflect.DeepEqual(settings, s.settings)
	if settingsUnchanged {
		s.settingsMu.Unlock()
		return "settings left unchanged"
	}
	newEnabled := *settings.Enabled
	previousEnabled := *s.settings.Enabled
	s.settings = settings
	s.settingsMu.Unlock()
	// Either restart or set changed status
	switch {
	case !newEnabled && !previousEnabled:
	case newEnabled && previousEnabled:
		_, _ = s.statusApplier.ApplyStatus(ctx, constants.Stopped)
		_, _ = s.statusApplier.ApplyStatus(ctx, constants.Running)
	case newEnabled && !previousEnabled:
		_, _ = s.statusApplier.ApplyStatus(ctx, constants.Running)
	case !newEnabled && previousEnabled:
		_, _ = s.statusApplier.ApplyStatus(ctx, constants.Stopped)
	}
	return "settings updated"
}
