package common

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
)

const gitTokenEnvName = "RURDESK_GIT_TOKEN"

// Reads the token from the git process env so the clone's config never holds a secret.
const credentialHelper = `!f() { test "$1" = get || exit 0; echo username=x-token; echo "password=$` + gitTokenEnvName + `"; }; f`

type AgentRunGitRepo struct {
	CloneUrl       string
	Host           string
	RepoPath       string
	BaseBranch     string
	EncryptedToken string
	Token          string
}

func (r AgentRunGitRepo) IsSet() bool {
	return r.CloneUrl != "" && r.Host != "" && r.RepoPath != "" && r.BaseBranch != "" && r.EncryptedToken != ""
}

func (r AgentRunGitRepo) LocalPath(workspaceBase string) (string, error) {
	if !isSafePathSegment(r.Host) {
		return "", fmt.Errorf("invalid repo host %q", r.Host)
	}
	segments := strings.Split(strings.Trim(r.RepoPath, "/"), "/")
	for _, segment := range segments {
		if !isSafePathSegment(segment) {
			return "", fmt.Errorf("invalid repo path %q", r.RepoPath)
		}
	}
	return filepath.Join(append([]string{workspaceBase, r.Host}, segments...)...), nil
}

func isSafePathSegment(segment string) bool {
	return segment != "" && segment != "." && segment != ".." && !strings.ContainsAny(segment, `/\`)
}

func GitTokenEnv(token string) string {
	return gitTokenEnvName + "=" + token
}

func ListClones(workspaceBase string) ([]string, error) {
	var clones []string
	err := filepath.WalkDir(workspaceBase, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return filepath.SkipAll
			}
			return err
		}
		if !entry.IsDir() || path == workspaceBase {
			return nil
		}
		if _, statErr := os.Stat(filepath.Join(path, ".git")); statErr == nil {
			clones = append(clones, path)
			return filepath.SkipDir
		}
		return nil
	})
	return clones, err
}

// Old single-repo clones hold the token in .git/config; new clones never sit directly under workspaceBase.
func RemoveLegacyClones(workspaceBase string) error {
	entries, err := os.ReadDir(workspaceBase)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading workspace: %w", err)
	}
	for _, entry := range entries {
		path := filepath.Join(workspaceBase, entry.Name())
		if !entry.IsDir() {
			continue
		}
		if _, statErr := os.Stat(filepath.Join(path, ".git")); statErr != nil {
			continue
		}
		log.Info().Str("path", path).Msg("removing a clone from the old single-repo layout")
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("removing old clone %s: %w", path, err)
		}
	}
	return nil
}
