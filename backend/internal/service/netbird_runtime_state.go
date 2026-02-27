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
	ConfiguredMode           NetBirdMode
	ConfiguredModeKnown      bool
	ConfiguredAllowLocalhost bool
	EffectiveMode            NetBirdMode
	EffectiveAllowLocalhost  bool
	Source                   string
	SourceJobID              uint
	Drift                    bool
	Warnings                 []string
}

func (s *NetBirdService) resolveNetBirdRuntimeState(ctx context.Context) netBirdRuntimeState {
	configuredMode, configuredKnown := configuredNetBirdMode(s.cfg.NetBirdMode)
	state := netBirdRuntimeState{
		ConfiguredMode:           configuredMode,
		ConfiguredModeKnown:      configuredKnown,
		ConfiguredAllowLocalhost: s.cfg.NetBirdAllowLocalhost,
		EffectiveMode:            configuredMode,
		EffectiveAllowLocalhost:  s.cfg.NetBirdAllowLocalhost,
		Source:                   netBirdModeSourceConfig,
		Warnings:                 []string{},
	}

	if !configuredKnown {
		state.Warnings = append(state.Warnings, "Configured mode is not valid; runtime assumed legacy mode.")
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

	mode, allowLocalhost, resolved, warning := runtimeStateFromSuccessfulSnapshot(snapshot)
	if warning != "" {
		state.Warnings = append(state.Warnings, warning)
	}
	if !resolved {
		return state
	}

	state.EffectiveMode = mode
	state.EffectiveAllowLocalhost = allowLocalhost
	state.Source = netBirdModeSourceLastSuccessfulSync
	state.SourceJobID = snapshot.Job.ID

	if state.ConfiguredMode != state.EffectiveMode || state.ConfiguredAllowLocalhost != state.EffectiveAllowLocalhost {
		state.Drift = true
		state.Warnings = append(
			state.Warnings,
			fmt.Sprintf(
				"Configured mode state (%s, allowLocalhost=%t) differs from the latest successful apply (%s, allowLocalhost=%t).",
				state.ConfiguredMode,
				state.ConfiguredAllowLocalhost,
				state.EffectiveMode,
				state.EffectiveAllowLocalhost,
			),
		)
	}

	return state
}

func runtimeStateFromSuccessfulSnapshot(snapshot netBirdModeApplySnapshot) (NetBirdMode, bool, bool, string) {
	if snapshot.Summary != nil {
		mode, err := ParseNetBirdMode(string(snapshot.Summary.TargetMode))
		if err == nil {
			return mode, snapshot.Summary.AllowLocalhost, true, ""
		}
		return NetBirdModeLegacy, false, false, "Latest successful NetBird apply summary had an invalid target mode; runtime fallback uses configured mode."
	}

	if snapshot.RequestParsed {
		mode, err := ParseNetBirdMode(snapshot.Request.TargetMode)
		if err == nil {
			return mode, snapshot.Request.AllowLocalhost, true, "Latest successful NetBird apply summary was missing; runtime mode was derived from the successful request payload."
		}
		return NetBirdModeLegacy, false, false, "Latest successful NetBird apply request had an invalid target mode; runtime fallback uses configured mode."
	}

	return NetBirdModeLegacy, false, false, "Latest successful NetBird apply payload could not be parsed; runtime fallback uses configured mode."
}
