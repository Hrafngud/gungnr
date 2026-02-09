package cloudflared

import "gungnr-cli/internal/cli/integrations/supervisor"

type PersistenceResult struct {
	RunScript    string
	EnsureScript string
	Supervisor   string
	Installed    bool
	Detail       string
}

func SetupAutoStart(configPath, stateDir string) (PersistenceResult, error) {
	result, err := supervisor.Setup(configPath, stateDir)
	if err != nil {
		return PersistenceResult{}, err
	}

	return PersistenceResult{
		RunScript:    result.RunScript,
		EnsureScript: result.EnsureScript,
		Supervisor:   string(result.Supervisor),
		Installed:    result.Installed,
		Detail:       result.Detail,
	}, nil
}
