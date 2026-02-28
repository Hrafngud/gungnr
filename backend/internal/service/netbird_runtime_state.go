package service

import (
	"context"
	"fmt"
)

const (
	netBirdModeSourceConfig             = "config"
	netBirdModeSourceLastSuccessfulSync = "last_successful_sync"
)

type netBirdRuntimeState struct {
	ConfiguredMode            NetBirdMode
	ConfiguredModeKnown       bool
	ConfiguredAllowLocalhost  bool
	ConfiguredModeBProjectIDs []uint
	EffectiveMode             NetBirdMode
	EffectiveAllowLocalhost   bool
	EffectiveModeBProjectIDs  []uint
	Source                    string
	SourceJobID               uint
	Drift                     bool
	Warnings                  []string
}

func (s *NetBirdService) resolveNetBirdRuntimeState(ctx context.Context) netBirdRuntimeState {
	configuredMode, configuredKnown := configuredNetBirdMode(s.cfg.NetBirdMode)
	state := netBirdRuntimeState{
		ConfiguredMode:            configuredMode,
		ConfiguredModeKnown:       configuredKnown,
		ConfiguredAllowLocalhost:  s.cfg.NetBirdAllowLocalhost,
		ConfiguredModeBProjectIDs: []uint{},
		EffectiveMode:             configuredMode,
		EffectiveAllowLocalhost:   s.cfg.NetBirdAllowLocalhost,
		EffectiveModeBProjectIDs:  []uint{},
		Source:                    netBirdModeSourceConfig,
		Warnings:                  []string{},
	}

	if !configuredKnown {
		state.Warnings = append(state.Warnings, "Configured mode is not valid; runtime assumed legacy mode.")
	}
	if s != nil && s.settings != nil {
		cfg, err := s.settings.GetNetBirdModeConfig(ctx)
		if err != nil {
			state.Warnings = append(state.Warnings, fmt.Sprintf("Failed to load configured Mode B project assignments: %v", err))
		} else {
			state.ConfiguredModeBProjectIDs = normalizeUintList(cfg.ModeBProjectIDs)
			state.EffectiveModeBProjectIDs = append([]uint(nil), state.ConfiguredModeBProjectIDs...)
		}
	}
	if s == nil || s.jobs == nil {
		return state
	}

	snapshot, err := s.latestSuccessfulModeApplySnapshot(ctx)
	if err != nil {
		state.Warnings = append(state.Warnings, fmt.Sprintf("Failed to load last successful NetBird apply state; runtime is using configured mode: %v", err))
		return state
	}
	if !snapshot.Found {
		return state
	}

	mode, allowLocalhost, modeBProjectIDs, resolved, warning := runtimeStateFromSuccessfulSnapshot(snapshot)
	if warning != "" {
		state.Warnings = append(state.Warnings, warning)
	}
	if !resolved {
		return state
	}

	state.EffectiveMode = mode
	state.EffectiveAllowLocalhost = allowLocalhost
	state.EffectiveModeBProjectIDs = normalizeUintList(modeBProjectIDs)
	state.Source = netBirdModeSourceLastSuccessfulSync
	state.SourceJobID = snapshot.Job.ID

	modeBSelectionDrift := (state.ConfiguredMode == NetBirdModeB || state.EffectiveMode == NetBirdModeB) &&
		!modeBProjectIDsEqual(state.ConfiguredModeBProjectIDs, state.EffectiveModeBProjectIDs)

	if state.ConfiguredMode != state.EffectiveMode ||
		state.ConfiguredAllowLocalhost != state.EffectiveAllowLocalhost ||
		modeBSelectionDrift {
		state.Drift = true
		state.Warnings = append(
			state.Warnings,
			fmt.Sprintf(
				"Configured mode state (%s, allowLocalhost=%t, modeBProjectIds=%v) differs from the latest successful apply (%s, allowLocalhost=%t, modeBProjectIds=%v).",
				state.ConfiguredMode,
				state.ConfiguredAllowLocalhost,
				state.ConfiguredModeBProjectIDs,
				state.EffectiveMode,
				state.EffectiveAllowLocalhost,
				state.EffectiveModeBProjectIDs,
			),
		)
	}

	return state
}

func runtimeStateFromSuccessfulSnapshot(snapshot netBirdModeApplySnapshot) (NetBirdMode, bool, []uint, bool, string) {
	if snapshot.Summary != nil {
		mode, err := ParseNetBirdMode(string(snapshot.Summary.TargetMode))
		if err == nil {
			return mode, snapshot.Summary.AllowLocalhost, normalizeUintList(snapshot.Summary.TargetModeBProjectIDs), true, ""
		}
		return NetBirdModeLegacy, false, nil, false, "Latest successful NetBird apply summary had an invalid target mode; runtime fallback uses configured mode."
	}

	if snapshot.RequestParsed {
		mode, err := ParseNetBirdMode(snapshot.Request.TargetMode)
		if err == nil {
			return mode, snapshot.Request.AllowLocalhost, normalizeUintList(snapshot.Request.ModeBProjectIDs), true, "Latest successful NetBird apply summary was missing; runtime mode was derived from the successful request payload."
		}
		return NetBirdModeLegacy, false, nil, false, "Latest successful NetBird apply request had an invalid target mode; runtime fallback uses configured mode."
	}

	return NetBirdModeLegacy, false, nil, false, "Latest successful NetBird apply payload could not be parsed; runtime fallback uses configured mode."
}

func modeBProjectIDsEqual(left []uint, right []uint) bool {
	l := normalizeUintList(left)
	r := normalizeUintList(right)
	if len(l) != len(r) {
		return false
	}
	for i := range l {
		if l[i] != r[i] {
			return false
		}
	}
	return true
}
