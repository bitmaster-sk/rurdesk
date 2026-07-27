package controller

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/githost"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
)

// fakeGitHost implements githost.GitHost for controller-level orchestration tests.
type fakeGitHost struct {
	findId, findUrl string
	found           bool
	findErr         error

	defaultBranch    string
	defaultBranchErr error

	createId, createUrl string
	createErr           error

	createCalls int
	createdBase string
}

func (f *fakeGitHost) GetMergeRequestChanges(context.Context, string) (*githost.Diff, error) {
	return nil, nil
}
func (f *fakeGitHost) GetMergeRequestStatus(context.Context, string) (*githost.Status, error) {
	return nil, nil
}
func (f *fakeGitHost) GetMergeRequestUrl(string) string { return "" }
func (f *fakeGitHost) DefaultBranch(context.Context) (string, error) {
	return f.defaultBranch, f.defaultBranchErr
}
func (f *fakeGitHost) FindOpenPullRequest(context.Context, string) (string, string, bool, error) {
	return f.findId, f.findUrl, f.found, f.findErr
}
func (f *fakeGitHost) CreatePullRequest(_ context.Context, _, base, _, _ string) (string, string, error) {
	f.createCalls++
	f.createdBase = base
	return f.createId, f.createUrl, f.createErr
}

func TestOpenOrReusePr(t *testing.T) {
	t.Run("reuses existing PR without creating", func(t *testing.T) {
		h := &fakeGitHost{found: true, findId: "42", findUrl: "u/42"}
		id, url, err := openOrReusePr(context.Background(), h, "feature", "t", "b")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != "42" || url != "u/42" {
			t.Errorf("got %q,%q want 42,u/42", id, url)
		}
		if h.createCalls != 0 {
			t.Errorf("expected no create call, got %d", h.createCalls)
		}
	})

	t.Run("creates against default branch when none found", func(t *testing.T) {
		h := &fakeGitHost{found: false, defaultBranch: "main", createId: "7", createUrl: "u/7"}
		id, url, err := openOrReusePr(context.Background(), h, "feature", "t", "b")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != "7" || url != "u/7" {
			t.Errorf("got %q,%q want 7,u/7", id, url)
		}
		if h.createCalls != 1 {
			t.Errorf("expected one create call, got %d", h.createCalls)
		}
		if h.createdBase != "main" {
			t.Errorf("expected base main, got %q", h.createdBase)
		}
	})

	t.Run("find error propagates, no create", func(t *testing.T) {
		h := &fakeGitHost{findErr: errors.New("boom")}
		_, _, err := openOrReusePr(context.Background(), h, "feature", "t", "b")
		if err == nil || !strings.Contains(err.Error(), "looking up existing PR") {
			t.Fatalf("expected lookup error, got %v", err)
		}
		if h.createCalls != 0 {
			t.Errorf("expected no create call, got %d", h.createCalls)
		}
	})

	t.Run("default-branch error propagates", func(t *testing.T) {
		h := &fakeGitHost{found: false, defaultBranchErr: errors.New("no repo")}
		_, _, err := openOrReusePr(context.Background(), h, "feature", "t", "b")
		if err == nil || !strings.Contains(err.Error(), "resolving base branch") {
			t.Fatalf("expected base branch error, got %v", err)
		}
		if h.createCalls != 0 {
			t.Errorf("expected no create call after base failure, got %d", h.createCalls)
		}
	})

	t.Run("create error propagates", func(t *testing.T) {
		h := &fakeGitHost{found: false, defaultBranch: "main", createErr: errors.New("422 no commits")}
		_, _, err := openOrReusePr(context.Background(), h, "feature", "t", "b")
		if err == nil || !strings.Contains(err.Error(), "opening PR") {
			t.Fatalf("expected open PR error, got %v", err)
		}
	})
}

func TestUniqueIntegrationForRepo(t *testing.T) {
	gh := &model.GitIntegration{IdGitIntegration: 1, RepoPath: "org/backend"}
	fe := &model.GitIntegration{IdGitIntegration: 2, RepoPath: "org/frontend"}
	dupA := &model.GitIntegration{IdGitIntegration: 3, RepoPath: "org/dup"}
	dupB := &model.GitIntegration{IdGitIntegration: 4, RepoPath: "org/dup"}

	cases := []struct {
		name     string
		ints     []*model.GitIntegration
		repoPath string
		wantId   int64 // 0 means nil expected
	}{
		{"unique match", []*model.GitIntegration{gh, fe}, "org/backend", 1},
		{"case insensitive", []*model.GitIntegration{gh, fe}, "ORG/Frontend", 2},
		{"slash trimmed", []*model.GitIntegration{gh}, "/org/backend/", 1},
		{"dot-git tolerated on input", []*model.GitIntegration{gh}, "org/backend.git", 1},
		{"no match", []*model.GitIntegration{gh, fe}, "org/other", 0},
		{"ambiguous", []*model.GitIntegration{dupA, dupB}, "org/dup", 0},
		{"empty list", nil, "org/backend", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := uniqueIntegrationForRepo(tc.ints, tc.repoPath)
			if tc.wantId == 0 {
				if got != nil {
					t.Fatalf("expected nil, got integration %d", got.IdGitIntegration)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected integration %d, got nil", tc.wantId)
			}
			if got.IdGitIntegration != tc.wantId {
				t.Errorf("got %d, want %d", got.IdGitIntegration, tc.wantId)
			}
		})
	}
}

func TestDerivePrHostAndId(t *testing.T) {
	cases := []struct {
		name         string
		url          string
		wantHostType string
		wantPrId     string
		wantErrPart  string
	}{
		{
			name:         "github cloud pull",
			url:          "https://github.com/bitmaster-sk/issue/pull/42",
			wantHostType: "github",
			wantPrId:     "42",
		},
		{
			name:         "gitlab merge request",
			url:          "https://gitlab.com/group/project/-/merge_requests/7",
			wantHostType: "gitlab",
			wantPrId:     "7",
		},
		{
			name:         "self-hosted github enterprise host",
			url:          "https://github.example.com/team/repo/pull/123",
			wantHostType: "github",
			wantPrId:     "123",
		},
		{
			name:         "gitea pull",
			url:          "https://gitea.example.org/team/repo/pulls/9",
			wantHostType: "gitea",
			wantPrId:     "9",
		},
		{
			name:         "unknown host but pull path infers github",
			url:          "https://forge.internal/team/repo/pull/3",
			wantHostType: "github",
			wantPrId:     "3",
		},
		{
			name:         "unknown host but merge_requests path infers gitlab",
			url:          "https://forge.internal/team/repo/-/merge_requests/4",
			wantHostType: "gitlab",
			wantPrId:     "4",
		},
		{
			name:        "url with no recognisable PR segment errors clearly",
			url:         "https://github.com/bitmaster-sk/issue",
			wantErrPart: "could not extract pr_id",
		},
		{
			name:        "github pull/new create-form URL is rejected (not numeric)",
			url:         "https://github.com/bitmaster-sk/issue/pull/new/agent/b1/i2/123",
			wantErrPart: "is not numeric",
		},
		{
			name:        "github pull/new without trailing branch is rejected",
			url:         "https://github.com/bitmaster-sk/issue/pull/new",
			wantErrPart: "is not numeric",
		},
		{
			name:        "invalid url errors out",
			url:         "::not-a-url::",
			wantErrPart: "invalid pr_url",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			host, id, err := derivePrHostAndId(tc.url)
			if tc.wantErrPart != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErrPart)
				}
				if !strings.Contains(err.Error(), tc.wantErrPart) {
					t.Errorf("expected error containing %q, got %q",
						tc.wantErrPart, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if host != tc.wantHostType {
				t.Errorf("hostType: got %q, want %q", host, tc.wantHostType)
			}
			if id != tc.wantPrId {
				t.Errorf("prId: got %q, want %q", id, tc.wantPrId)
			}
		})
	}
}
