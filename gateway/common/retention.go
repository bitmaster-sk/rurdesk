package common

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
)

// Retention deletes worktree directories idle longer than maxAge. Idle is
// measured by mtime — the agent touches files on every stage attempt, so no
// recent activity means complete or abandoned. Worktrees active in the
// orchestrator's activeTasks map are skipped regardless of mtime.
type Retention struct {
	cfg          *Config
	orchestrator *Orchestrator
	interval     time.Duration
	maxAge       time.Duration
}

func NewRetention(cfg *Config, orch *Orchestrator) *Retention {
	return &Retention{
		cfg:          cfg,
		orchestrator: orch,
		interval:     1 * time.Hour,
		maxAge:       7 * 24 * time.Hour,
	}
}

func (r *Retention) Start(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	// Run once on startup so a stale worktree isn't left for an hour before
	// the first sweep.
	r.runOnce()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.runOnce()
		}
	}
}

func (r *Retention) runOnce() {
	activeRuns := r.orchestrator.activeRunIDs()
	cutoff := time.Now().Add(-r.maxAge)

	repoPath := RepoPathFromURL(r.cfg.WorkspaceBase, r.cfg.RepoUrl)
	agentRunsPath := filepath.Join(repoPath, agentRunsDir)
	entries, err := os.ReadDir(agentRunsPath)
	if err != nil {
		// Missing simply means no run has created a worktree yet.
		if !os.IsNotExist(err) {
			log.Warn().Str("path", agentRunsPath).Err(err).Msg("retention: cannot read worktree dir")
		}
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		idRun, parseErr := strconv.ParseInt(entry.Name(), 10, 64)
		if parseErr != nil {
			continue
		}
		if activeRuns[idRun] {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(cutoff) {
			continue
		}
		log.Info().
			Int64("idRun", idRun).
			Int("ageHours", int(time.Since(info.ModTime()).Hours())).
			Msg("retention: pruning idle worktree")
		if err := RemoveWorktree(repoPath, idRun); err != nil {
			log.Warn().Int64("idRun", idRun).Err(err).Msg("retention: prune failed")
		}
	}
}
