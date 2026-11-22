package common

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// prepareWorkspace returns a cleanup only for the scratch directory of a
// repoless stage; a run's worktree outlives the stage attempt.
func (o *Orchestrator) prepareWorkspace(task *Task) (cleanup func(), ok bool) {
	if task.Stage == StageReleaseNotes || (task.Stage == StageChat && !task.Repo.IsSet()) {
		return o.prepareScratchDir(task)
	}
	if task.Stage == StageChat {
		hasCode, ok := o.prepareChatWorktree(task)
		if ok && !hasCode {
			return o.prepareScratchDir(task)
		}
		return nil, ok
	}

	repoPath, err := EnsureRepo(o.cfg.WorkspaceBase, task.Repo)
	if err != nil {
		log.Error().Int64("idTask", task.IdTask).Err(err).Msg("failed to prepare the repo")
		o.failTask(*task, "worktree_error", err.Error())
		return nil, false
	}

	isReused, ok := o.reuseRunWorktree(repoPath, task)
	if !ok {
		return nil, false
	}
	// A run without a worktree resumes the branch of the issue's open PR, or starts a new one.
	if !isReused {
		path, branch, err := o.createRunWorktree(repoPath, *task)
		if err != nil {
			log.Error().Int64("idTask", task.IdTask).Err(err).Msg("failed to create worktree")
			o.failTask(*task, "worktree_error", err.Error())
			return nil, false
		}
		task.WorktreePath = path
		task.Branch = branch
	}
	markWorktreeUsed(*task)
	return nil, true
}

func (o *Orchestrator) prepareScratchDir(task *Task) (cleanup func(), ok bool) {
	dir, err := os.MkdirTemp("", fmt.Sprintf("%s-%d-", strings.ReplaceAll(task.Stage, "_", "-"), task.IdRun))
	if err != nil {
		log.Error().Int64("idTask", task.IdTask).Err(err).Msg("failed to create the scratch directory")
		o.failTask(*task, "worktree_error", err.Error())
		return nil, false
	}
	task.WorktreePath = dir
	return func() { _ = os.RemoveAll(dir) }, true
}

// markWorktreeUsed touches the worktree root: retention measures idleness by
// its mtime, which edits in subdirectories never change.
func markWorktreeUsed(task Task) {
	now := time.Now()
	if err := os.Chtimes(task.WorktreePath, now, now); err != nil {
		log.Warn().Int64("idRun", task.IdRun).Err(err).Msg("failed to mark the worktree as used")
	}
}

// prepareChatWorktree reuses the chat run's worktree without fetching, reset to the commit the
// tracker recorded, so nothing the agent did survives the turn. Without a usable recorded commit
// it is (re)created detached at the current origin/<base>; hasCode is false for an empty origin.
func (o *Orchestrator) prepareChatWorktree(task *Task) (hasCode, ok bool) {
	repoPath, err := task.Repo.LocalPath(o.cfg.WorkspaceBase)
	if err != nil {
		log.Error().Int64("idTask", task.IdTask).Err(err).Msg("invalid chat repo")
		o.failTask(*task, "worktree_error", err.Error())
		return false, false
	}
	worktreePath := WorktreePath(repoPath, task.IdRun)
	head := task.Chat.HeadCommit
	if err := ResetDetachedWorktree(worktreePath, head); err != nil {
		if WorktreeExists(repoPath, task.IdRun) {
			log.Warn().Int64("idRun", task.IdRun).Err(err).Msg("chat worktree cannot be reused — creating a new one")
			if err := RemoveWorktree(repoPath, task.IdRun); err != nil {
				log.Error().Int64("idTask", task.IdTask).Err(err).Msg("failed to remove the chat worktree")
				o.failTask(*task, "worktree_error", err.Error())
				return false, false
			}
		}
		head, err = o.createReadOnlyWorktree(task.Repo, task.IdRun)
		if errors.Is(err, ErrRemoteEmpty) {
			log.Info().Int64("idRun", task.IdRun).Msg("chat repo is empty — running without code")
			return false, true
		}
		if err != nil {
			log.Error().Int64("idTask", task.IdTask).Err(err).Msg("failed to create the chat worktree")
			o.failTask(*task, "worktree_error", err.Error())
			return false, false
		}
	}
	task.WorktreePath = worktreePath
	task.Chat.WorktreeHead = head
	markWorktreeUsed(*task)
	return true, true
}

// createReadOnlyWorktree creates a detached worktree from a clone that is never
// seeded: an empty origin fails with ErrRemoteEmpty.
func (o *Orchestrator) createReadOnlyWorktree(repo AgentRunGitRepo, idRun int64) (head string, err error) {
	repoPath, err := EnsureRepoReadOnly(o.cfg.WorkspaceBase, repo)
	if err != nil {
		return "", err
	}
	return CreateDetachedWorktree(repoPath, repo.Token, repo.BaseBranch, idRun)
}

// reuseRunWorktree points the task at the run's existing worktree. A clean
// detached worktree is removed so the caller creates a new one; any other
// worktree without a branch fails the stage and stays on disk for an operator.
func (o *Orchestrator) reuseRunWorktree(repoPath string, task *Task) (isReused, ok bool) {
	if !WorktreeExists(repoPath, task.IdRun) {
		return false, true
	}
	worktreePath := WorktreePath(repoPath, task.IdRun)
	branch, isDisposable, err := worktreeState(worktreePath)
	switch {
	case err == nil && branch != "":
		task.WorktreePath = worktreePath
		task.Branch = branch
		task.ResumedWorktree = true
		return true, true
	case err == nil && isDisposable:
		log.Info().Int64("idRun", task.IdRun).Msg("worktree is detached and holds no work — creating a new one")
		if err := RemoveWorktree(repoPath, task.IdRun); err != nil {
			log.Error().Int64("idTask", task.IdTask).Err(err).Msg("failed to remove the detached worktree")
			o.failTask(*task, "worktree_error", err.Error())
			return false, false
		}
		return false, true
	default:
		log.Warn().Int64("idTask", task.IdTask).Str("path", worktreePath).AnErr("cause", err).Msg("worktree cannot be reused")
		o.failTask(*task, ErrCodeWorktreeUnusable, fmt.Sprintf(
			"the worktree of run %d at %s has no branch checked out or is not a git worktree, so this run cannot continue on it; "+
				"another run has likely taken over its branch. To retry, remove that directory on the gateway host; anything not pushed is then lost",
			task.IdRun, worktreePath))
		return false, false
	}
}

func (o *Orchestrator) createRunWorktree(repoPath string, task Task) (path, branch string, err error) {
	if o.canResumePrBranch(repoPath, task) {
		path, err = ResumeWorktree(repoPath, task.Repo.Token, task.PrBranch, task.IdRun)
		return path, task.PrBranch, err
	}
	branch = GenerateBranchName(task.IdUserAgent, task.IdIssue)
	path, err = CreateWorktree(repoPath, task.Repo.Token, task.Repo.BaseBranch, branch, task.IdRun)
	return path, branch, err
}

func (o *Orchestrator) canResumePrBranch(repoPath string, task Task) bool {
	if task.PrBranch == "" {
		return false
	}
	if !IsAgentBranch(task.PrBranch) {
		log.Warn().Int64("idRun", task.IdRun).Str("branch", task.PrBranch).Msg("PR branch is not an agent branch — starting a new one")
		return false
	}
	hasBranch, err := remoteHasBranch(repoPath, task.Repo.Token, task.PrBranch)
	if err != nil {
		log.Warn().Int64("idRun", task.IdRun).Str("branch", task.PrBranch).Err(err).Msg("checking PR branch on origin failed — starting a new one")
		return false
	}
	return hasBranch
}
