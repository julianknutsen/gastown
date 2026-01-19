package agent

import (
	"fmt"
	"time"
)

// StartConfig holds configuration for starting an agent.
// This allows per-Start configuration instead of per-Agents configuration.
// The key benefit is callbacks can capture context via closures rather than
// requiring managers to stash mutable state.
type StartConfig struct {
	// WorkDir is the working directory for the agent.
	WorkDir string

	// Command is the command to execute.
	Command string

	// EnvVars are environment variables to prepend to the command.
	// These are merged with any EnvVars configured at the Agents level.
	EnvVars map[string]string

	// OnCreated is called immediately after session creation.
	// This is per-start (unlike Config.OnSessionCreated which is per-Agents).
	// Use for setup that depends on start-time context like agent name.
	OnCreated OnSessionCreated
}

// mergeEnvVars combines Agents-level and Start-level env vars.
// Start-level vars override Agents-level vars with the same key.
func mergeEnvVars(agentsVars, startVars map[string]string) map[string]string {
	if len(agentsVars) == 0 && len(startVars) == 0 {
		return nil
	}
	result := make(map[string]string)
	for k, v := range agentsVars {
		result[k] = v
	}
	for k, v := range startVars {
		result[k] = v
	}
	return result
}

// StartWithConfig launches an agent process with explicit configuration.
// This is more testable than Start() because all configuration is explicit.
//
// Differences from Start():
//   - WorkDir and Command are in StartConfig instead of separate parameters
//   - EnvVars from StartConfig are merged with Agents-level EnvVars
//   - OnCreated callback is per-start (can capture context via closure)
func (a *Implementation) StartWithConfig(id AgentID, cfg StartConfig) error {
	sessionID := a.sess.SessionIDForAgent(id)
	debugf("[%s] agent.StartWithConfig: id=%s sessionID=%s workDir=%s\n",
		time.Now().Format("15:04:05.000"), id, sessionID, cfg.WorkDir)
	debugf("[%s] agent.StartWithConfig: command=%s\n",
		time.Now().Format("15:04:05.000"), cfg.Command)

	// Check for existing session and handle zombie detection
	debugf("[%s] agent.StartWithConfig: checking for existing session\n",
		time.Now().Format("15:04:05.000"))
	exists, _ := a.sess.Exists(sessionID)
	if exists {
		debugf("[%s] agent.StartWithConfig: session exists, checking if running\n",
			time.Now().Format("15:04:05.000"))
		// Session exists - check if agent is actually running (healthy vs zombie)
		if a.sess.IsRunning(sessionID, a.config.ProcessNames...) {
			debugf("[%s] agent.StartWithConfig: agent already running, returning error\n",
				time.Now().Format("15:04:05.000"))
			return ErrAlreadyRunning
		}
		// Zombie - session alive but agent dead. Kill and recreate.
		debugf("[%s] agent.StartWithConfig: zombie detected, killing session\n",
			time.Now().Format("15:04:05.000"))
		if err := a.sess.Stop(sessionID); err != nil {
			return fmt.Errorf("killing zombie session: %w", err)
		}
	}

	// Merge env vars: Agents-level + Start-level (Start-level wins on conflict)
	envVars := mergeEnvVars(a.config.EnvVars, cfg.EnvVars)

	// Build final command with env vars prepended
	command := cfg.Command
	if len(envVars) > 0 {
		command = prependEnvVars(envVars, command)
	}

	// Create the session
	debugf("[%s] agent.StartWithConfig: calling sess.Start\n",
		time.Now().Format("15:04:05.000"))
	if _, err := a.sess.Start(string(sessionID), cfg.WorkDir, command); err != nil {
		debugf("[%s] agent.StartWithConfig: sess.Start FAILED: %v\n",
			time.Now().Format("15:04:05.000"), err)
		return fmt.Errorf("starting session: %w", err)
	}
	debugf("[%s] agent.StartWithConfig: sess.Start succeeded\n",
		time.Now().Format("15:04:05.000"))

	// Run Agents-level callback first (if any)
	if a.config.OnSessionCreated != nil {
		debugf("[%s] agent.StartWithConfig: running Agents-level OnSessionCreated\n",
			time.Now().Format("15:04:05.000"))
		if err := a.config.OnSessionCreated(sessionID); err != nil {
			_ = a.sess.Stop(sessionID)
			return fmt.Errorf("session setup: %w", err)
		}
	}

	// Run per-start callback (if any)
	if cfg.OnCreated != nil {
		debugf("[%s] agent.StartWithConfig: running per-start OnCreated callback\n",
			time.Now().Format("15:04:05.000"))
		if err := cfg.OnCreated(sessionID); err != nil {
			_ = a.sess.Stop(sessionID)
			return fmt.Errorf("session setup: %w", err)
		}
	}

	// Wait for agent to be ready (non-blocking)
	debugf("[%s] agent.StartWithConfig: spawning background doWaitForReady goroutine\n",
		time.Now().Format("15:04:05.000"))
	go a.doWaitForReady(id)

	debugf("[%s] agent.StartWithConfig: returning success\n",
		time.Now().Format("15:04:05.000"))
	return nil
}
