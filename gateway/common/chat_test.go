package common

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/cgi"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const testRunToken = "rdr_0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// chatTracker is a tracker stub that answers every call with 200 and keeps the
// stats and complete_stage bodies by path.
type chatTracker struct {
	mu     sync.Mutex
	bodies map[string]map[string]any
}

func newChatTracker(t *testing.T) (*chatTracker, string) {
	t.Helper()
	tracker := &chatTracker{bodies: map[string]map[string]any{}}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err == nil {
			tracker.mu.Lock()
			tracker.bodies[request.URL.Path] = body
			tracker.mu.Unlock()
		}
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	return tracker, server.URL
}

func (c *chatTracker) body(path string) map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.bodies[path]
}

func chatTestOrchestrator(t *testing.T, agent Agent) (*Orchestrator, *chatTracker, AgentRunGitRepo, string) {
	t.Helper()
	tmp := newGitEnv(t)
	originPath, seedPath := seedOrigin(t, tmp, "main")
	tracker, trackerURL := newChatTracker(t)
	cfg := &Config{TrackerAPIUrl: trackerURL, WebhookSecret: testWebhookSecret, WorkspaceBase: filepath.Join(tmp, "ws")}
	return NewOrchestrator(cfg, agent, NewTrackerClient(cfg), NewState()), tracker, testRepo(originPath, "example.com", "main"), seedPath
}

func chatTask(repo AgentRunGitRepo, idTask int64) Task {
	return Task{
		Repo:      repo,
		IdTask:    idTask,
		IdRun:     500,
		Stage:     StageChat,
		AttemptNo: 1,
		RunToken:  testRunToken,
		Chat:      &ChatContext{IdConversation: 9, Mode: ChatModeReply},
	}
}

// chatTaskAt is a later turn of the conversation: the tracker sends the commit
// the previous turn reported.
func chatTaskAt(repo AgentRunGitRepo, idTask int64, headCommit string) Task {
	task := chatTask(repo, idTask)
	task.Chat.HeadCommit = headCommit
	return task
}

func pushCommit(t *testing.T, seedPath, name string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(seedPath, name), []byte(name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, seedPath, "add", name)
	git(t, seedPath, "commit", "-m", name)
	git(t, seedPath, "push", "origin", "main")
	return headSha(t, seedPath)
}

func TestRunStage_Chat_ReusesExistingWorktreeWithoutFetch(t *testing.T) {
	agent := &recordingAgent{}
	orchestrator, tracker, repo, seedPath := chatTestOrchestrator(t, agent)
	orchestrator.runStage(chatTask(repo, 100))
	worktreePath := runWorktreePath(t, orchestrator, repo, 500)
	firstHead := headSha(t, worktreePath)
	idle := time.Now().Add(-3 * 24 * time.Hour)
	if err := os.Chtimes(worktreePath, idle, idle); err != nil {
		t.Fatal(err)
	}
	pushCommit(t, seedPath, "later.txt")
	// An unreachable origin proves the reuse never talks to it.
	if err := os.Rename(repo.CloneUrl, repo.CloneUrl+".gone"); err != nil {
		t.Fatal(err)
	}

	orchestrator.runStage(chatTaskAt(repo, 101, firstHead))

	ran := agent.ran()
	if len(ran) != 2 {
		t.Fatalf("agent ran %d times, want 2 (complete: %v)", len(ran), tracker.body("/agent/task/101/complete"))
	}
	if ran[1].WorktreePath != worktreePath {
		t.Errorf("second turn ran in %q, want the existing worktree %q", ran[1].WorktreePath, worktreePath)
	}
	if got := headSha(t, worktreePath); got != firstHead {
		t.Errorf("worktree HEAD moved to %s, want %s kept", got, firstHead)
	}
	if ran[1].Chat.WorktreeHead != firstHead {
		t.Errorf("the turn was told HEAD %q, want %q", ran[1].Chat.WorktreeHead, firstHead)
	}
	info, err := os.Stat(worktreePath)
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(info.ModTime()) > time.Hour {
		t.Errorf("reused worktree mtime %v was not touched, retention would prune it", info.ModTime())
	}
}

func TestRunStage_Chat_RecreatesDetachedAtCurrentDefault_ReportsHeadCommit(t *testing.T) {
	agent := &recordingAgent{}
	orchestrator, tracker, repo, seedPath := chatTestOrchestrator(t, agent)
	orchestrator.runStage(chatTask(repo, 100))
	worktreePath := runWorktreePath(t, orchestrator, repo, 500)
	firstHead := headSha(t, worktreePath)
	if body := tracker.body("/agent/task/100/stats"); body == nil || body["headCommit"] != firstHead {
		t.Errorf("first turn stats = %v, want headCommit %s", body, firstHead)
	}
	newHead := pushCommit(t, seedPath, "later.txt")
	if err := RemoveWorktree(filepath.Dir(filepath.Dir(worktreePath)), 500); err != nil {
		t.Fatal(err)
	}

	orchestrator.runStage(chatTask(repo, 101))

	if got := headSha(t, worktreePath); got != newHead {
		t.Errorf("recreated worktree HEAD = %s, want the current origin/main %s", got, newHead)
	}
	if branch, err := branchOfWorktree(worktreePath); err != nil || branch != "" {
		t.Errorf("chat worktree is on branch %q (err %v), want a detached HEAD", branch, err)
	}
	body := tracker.body("/agent/task/101/stats")
	if body == nil || body["headCommit"] != newHead {
		t.Errorf("stats = %v, want headCommit %s", body, newHead)
	}
}

func TestRunStage_Chat_NoRepo_UsesScratchDir(t *testing.T) {
	tracker, trackerURL := newChatTracker(t)
	cfg := &Config{WebhookSecret: testWebhookSecret, TrackerAPIUrl: trackerURL, WorkspaceBase: filepath.Join(t.TempDir(), "never-created")}
	agent := &workspaceRecordingAgent{workspace: make(chan string, 1), entries: make(chan int, 1)}
	orchestrator := NewOrchestrator(cfg, agent, NewTrackerClient(cfg), NewState())

	orchestrator.runStage(chatTask(AgentRunGitRepo{}, 100))

	select {
	case workspace := <-agent.workspace:
		if workspace == "" {
			t.Fatal("the agent got no working directory")
		}
		if entries := <-agent.entries; entries != 0 {
			t.Errorf("the scratch directory held %d entries, want none", entries)
		}
		if !isMissing(workspace) {
			t.Errorf("the scratch directory %s is left behind", workspace)
		}
	default:
		t.Fatalf("the agent never ran: a chat without a repo must not be rejected (complete: %v)", tracker.body("/agent/task/100/complete"))
	}
	if !isMissing(cfg.WorkspaceBase) {
		t.Error("a chat without a repo must not touch the repo workspace")
	}
	if body := tracker.body("/agent/task/100/stats"); body != nil && body["headCommit"] != nil {
		t.Errorf("a chat without a repo reported headCommit %v", body["headCommit"])
	}
}

func TestRunStage_Chat_FailsWithoutRunToken(t *testing.T) {
	agent := &recordingAgent{}
	orchestrator, tracker, repo, _ := chatTestOrchestrator(t, agent)
	task := chatTask(repo, 100)
	task.RunToken = ""

	orchestrator.runStage(task)

	if len(agent.ran()) != 0 {
		t.Fatal("a chat turn without its run token ran the agent")
	}
	body := tracker.body("/agent/task/100/complete")
	if body == nil || body["outcome"] != OutcomeErrored || body["errorReason"] != ErrCodeTrackerMissingRunToken {
		t.Errorf("complete = %v, want errored with %s", body, ErrCodeTrackerMissingRunToken)
	}
}

type leakingAgent struct{}

func (leakingAgent) Run(_ context.Context, task Task) (RunStats, error) {
	return RunStats{}, fmt.Errorf("mcp said: bad token %s", task.RunToken)
}

func (leakingAgent) Cancel(_ context.Context, _ RunID) error { return nil }

func TestRunStage_Chat_RedactsTheRunToken(t *testing.T) {
	orchestrator, tracker, _, _ := chatTestOrchestrator(t, leakingAgent{})

	orchestrator.runStage(chatTask(AgentRunGitRepo{}, 100))

	body := tracker.body("/agent/task/100/complete")
	if body == nil {
		t.Fatal("the failed turn was not reported")
	}
	detail, _ := body["errorDetail"].(string)
	if strings.Contains(detail, testRunToken) || !strings.Contains(detail, "***") {
		t.Errorf("errorDetail = %q, want the run token masked", detail)
	}
}

func TestRunStage_ChatQueuesBehindStage(t *testing.T) {
	workspaceBase, repo := stageTestWorkspace(t)
	_, trackerURL := newChatTracker(t)
	cfg := &Config{WebhookSecret: testWebhookSecret, WorkspaceBase: workspaceBase, TrackerAPIUrl: trackerURL}
	agent := &blockingAgent{entered: make(chan int64, 2), release: make(chan struct{})}
	orchestrator := NewOrchestrator(cfg, agent, NewTrackerClient(cfg), NewState())

	go orchestrator.runStage(Task{Repo: repo, IdTask: 100, IdRun: 200, IdUserAgent: 1, IdIssue: 300, Stage: StageDesign, AttemptNo: 1})
	select {
	case id := <-agent.entered:
		if id != 100 {
			t.Fatalf("task %d started first, want the issue stage 100", id)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the issue stage never started")
	}
	go orchestrator.runStage(chatTask(repo, 101))
	select {
	case id := <-agent.entered:
		t.Fatalf("task %d started while the issue stage held the only slot", id)
	case <-time.After(300 * time.Millisecond):
	}
	if got := orchestrator.ActiveTaskCount(); got != 2 {
		t.Errorf("active tasks = %d, want the queued chat turn counted (2)", got)
	}

	close(agent.release)
	select {
	case id := <-agent.entered:
		if id != 101 {
			t.Errorf("task %d started, want the chat turn 101", id)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the chat turn never started after the stage finished")
	}
}

func TestHandleEvent_RunDiscardedRemovesChatWorktree(t *testing.T) {
	orchestrator, _, repo, _ := chatTestOrchestrator(t, &recordingAgent{})
	orchestrator.runStage(chatTask(repo, 100))
	worktreePath := runWorktreePath(t, orchestrator, repo, 500)
	if isMissing(worktreePath) {
		t.Fatal("the chat turn created no worktree")
	}

	if err := orchestrator.HandleEvent(WebhookEvent{Type: "run_discarded", IdRun: 500}); err != nil {
		t.Fatalf("HandleEvent(run_discarded): %v", err)
	}

	waitUntil(t, "the deleted conversation's worktree is removed", func() bool { return isMissing(worktreePath) })
}

func TestRetention_PrunesIdleChatWorktree(t *testing.T) {
	orchestrator, _, repo, _ := chatTestOrchestrator(t, &recordingAgent{})
	orchestrator.cfg.WorktreeRetention = 24 * time.Hour
	orchestrator.runStage(chatTask(repo, 100))
	worktreePath := runWorktreePath(t, orchestrator, repo, 500)
	idle := time.Now().Add(-2 * 24 * time.Hour)
	if err := os.Chtimes(worktreePath, idle, idle); err != nil {
		t.Fatal(err)
	}

	NewRetention(orchestrator.cfg, orchestrator).runOnce()

	if !isMissing(worktreePath) {
		t.Error("a chat worktree idle for 2 days survived a 1-day retention")
	}
}

func TestCreateDetachedWorktree_StartsAtCurrentOriginBase(t *testing.T) {
	tmp := newGitEnv(t)
	originPath, seedPath := seedOrigin(t, tmp, "main")
	repoPath := ensureRepo(t, filepath.Join(tmp, "ws"), testRepo(originPath, "example.com", "main"))
	newHead := pushCommit(t, seedPath, "after-clone.txt")

	head, err := CreateDetachedWorktree(repoPath, testGitToken, "main", 7)
	if err != nil {
		t.Fatalf("CreateDetachedWorktree: %v", err)
	}

	worktreePath := WorktreePath(repoPath, 7)
	if head != newHead || headSha(t, worktreePath) != newHead {
		t.Errorf("head = %s (worktree %s), want the current origin/main %s", head, headSha(t, worktreePath), newHead)
	}
	if branch, err := branchOfWorktree(worktreePath); err != nil || branch != "" {
		t.Errorf("worktree is on branch %q (err %v), want a detached HEAD", branch, err)
	}
}

func TestAgentEnv_StripsTrackerAndGitTokens(t *testing.T) {
	base := []string{
		"PATH=/usr/bin",
		"GATEWAY_TO_TRACKER_TOKEN=agent-key",
		"TRACKER_TO_GATEWAY_TOKEN=webhook-secret",
		"RURDESK_GIT_TOKEN=inherited-git-token",
		"HOME=/root",
	}
	for _, tt := range []struct {
		name     string
		gitToken string
		want     []string
	}{
		{"chat gets no git token", "", []string{"PATH=/usr/bin", "HOME=/root"}},
		{"issue stage gets the git token of its run", "run-token", []string{"PATH=/usr/bin", "HOME=/root", "RURDESK_GIT_TOKEN=run-token"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			env := AgentEnv(base, tt.gitToken)
			if strings.Join(env, "\n") != strings.Join(tt.want, "\n") {
				t.Errorf("AgentEnv = %q, want %q", env, tt.want)
			}
		})
	}
}

func TestTaskTokens_ChatNeverGetsTheAgentKeyOrTheGitToken(t *testing.T) {
	chat := Task{Stage: StageChat, RunToken: testRunToken, Repo: AgentRunGitRepo{Token: "git-token"}}
	if got := chat.MCPBearer("agent-key"); got != testRunToken {
		t.Errorf("chat MCP bearer = %q, want the run token", got)
	}
	if got := (Task{Stage: StageChat}).MCPBearer("agent-key"); got != "" {
		t.Errorf("chat without a run token fell back to %q", got)
	}
	if got := chat.AgentGitToken(); got != "" {
		t.Errorf("chat agent git token = %q, want none", got)
	}
	issue := Task{Stage: StageImplementation, RunToken: testRunToken, Repo: AgentRunGitRepo{Token: "git-token"}}
	if got := issue.MCPBearer("agent-key"); got != "agent-key" {
		t.Errorf("issue MCP bearer = %q, want the agent key", got)
	}
	if got := issue.AgentGitToken(); got != "git-token" {
		t.Errorf("issue agent git token = %q, want the run's token", got)
	}
}

// authGitServer serves the bare repos under root over smart HTTP and accepts
// only the x-token user with token, like a git host does.
func authGitServer(t *testing.T, root, token string) string {
	t.Helper()
	execPath, err := exec.Command("git", "--exec-path").Output()
	if err != nil {
		t.Fatalf("git --exec-path: %v", err)
	}
	// A credential helper outside the test env (a keychain) would be handed the
	// test token: never run these tests where one is still in effect.
	if helpers, _ := runGitQuietOutput(root, "config", "--get-all", "credential.helper"); strings.TrimSpace(helpers) != "" {
		t.Skipf("a credential helper outside the test env is in effect: %q", strings.TrimSpace(helpers))
	}
	backend := filepath.Join(strings.TrimSpace(string(execPath)), "git-http-backend")
	if _, err := os.Stat(backend); err != nil {
		t.Skipf("git-http-backend not installed: %v", err)
	}
	handler := &cgi.Handler{
		Path: backend,
		Env:  []string{"GIT_PROJECT_ROOT=" + root, "GIT_HTTP_EXPORT_ALL=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_SYSTEM=" + os.DevNull},
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		user, password, ok := request.BasicAuth()
		if !ok || user != "x-token" || password != token {
			writer.Header().Set("WWW-Authenticate", `Basic realm="git"`)
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		handler.ServeHTTP(writer, request)
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func TestChatWorktree_PushFails(t *testing.T) {
	tmp := newGitEnv(t)
	originPath, _ := seedOrigin(t, tmp, "main")
	git(t, originPath, "config", "http.receivepack", "true")
	repo := testRepo(authGitServer(t, tmp, testGitToken)+"/origin.git", "example.com", "main")
	repoPath := ensureRepo(t, filepath.Join(tmp, "ws"), repo)
	if _, err := CreateDetachedWorktree(repoPath, testGitToken, "main", 7); err != nil {
		t.Fatalf("CreateDetachedWorktree: %v", err)
	}
	worktreePath := WorktreePath(repoPath, 7)
	if err := os.WriteFile(filepath.Join(worktreePath, "agent.txt"), []byte("change\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, worktreePath, "add", "agent.txt")
	git(t, worktreePath, "commit", "-m", "agent change")
	push := func(env []string) error {
		cmd := exec.Command("git", "push", "origin", "HEAD:refs/heads/agent/a1/i1/1")
		cmd.Dir = worktreePath
		cmd.Env = append(env, "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=", "SSH_ASKPASS=")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%w: %s", err, out)
		}
		return nil
	}
	t.Setenv("GATEWAY_TO_TRACKER_TOKEN", "agent-key")
	t.Setenv(gitTokenEnvName, testGitToken)

	if err := push(AgentEnv(os.Environ(), "")); err == nil {
		t.Fatal("a push from the chat worktree with the chat agent env succeeded")
	}
	if out, _ := runGitQuietOutput(originPath, "branch", "--list", "agent/*"); strings.TrimSpace(out) != "" {
		t.Errorf("origin got the branch %q from the chat worktree", out)
	}
	// The same push with the token in the env goes through, so the missing
	// token is what stopped it.
	if err := push(AgentEnv(os.Environ(), testGitToken)); err != nil {
		t.Fatalf("control push with the git token failed: %v", err)
	}
	assertNoFileHolds(t, filepath.Join(tmp, "ws"), testGitToken)
}

func assertNoFileHolds(t *testing.T, dir, secret string) {
	t.Helper()
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(content), secret) {
			t.Errorf("the git token is written to %s", path)
		}
		return nil
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}

func TestParseStageExecutePayload_Chat(t *testing.T) {
	event := unmarshalEvent(t, `{
		"idRun": 42,
		"idUserAgent": 84,
		"idProject": 1,
		"payload": {
			"idTask": 123,
			"stage": "chat",
			"attemptNo": 12,
			"runToken": "`+testRunToken+`",
			"contextBundle": {
				"agent": {"name": "Agent 007"},
				"historyTruncated": true,
				"reviewThread": [
					{"idMessage": 1, "idConversation": 9, "author": "user", "kind": "text", "body": "How does login work?", "idUser": 5, "idTask": null, "hasThinking": false, "createdAt": "2026-11-16T10:00:00Z"},
					{"idMessage": 2, "idConversation": 9, "author": "agent", "kind": "text", "body": "Through the auth middleware.", "idUser": null, "idTask": 122, "hasThinking": true, "createdAt": "2026-11-16T10:01:00Z"}
				],
				"chat": {
					"idConversation": 9,
					"mode": "task_draft",
					"projects": [{"idProject": 1, "name": "Rurdesk", "isMain": true}, {"idProject": 3, "name": "Docs", "isMain": false}],
					"headCommit": null,
					"draftFromMessage": {"idMessage": 2, "body": "Through the auth middleware."}
				}
			}
		}
	}`)

	task, err := parseStageExecutePayload(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.RunToken != testRunToken {
		t.Errorf("RunToken = %q, want the run token", task.RunToken)
	}
	if task.Chat == nil {
		t.Fatal("chat context not parsed")
	}
	want := ChatContext{IdConversation: 9, Mode: ChatModeTaskDraft, IsHistoryTruncated: true, Projects: []ChatProject{{1, "Rurdesk", true}, {3, "Docs", false}}}
	if task.Chat.DraftFromMessage == nil || *task.Chat.DraftFromMessage != (ChatDraftFromMessage{IdMessage: 2, Body: "Through the auth middleware."}) {
		t.Errorf("draftFromMessage = %+v", task.Chat.DraftFromMessage)
	}
	chat := *task.Chat
	chat.DraftFromMessage = nil
	if fmt.Sprint(chat) != fmt.Sprint(want) {
		t.Errorf("chat = %+v, want %+v", chat, want)
	}
	if len(task.Conversation) != 2 ||
		task.Conversation[0] != (TaskMessage{CreatorName: "user", Kind: "text", CreatedAt: "2026-11-16T10:00:00Z", Message: "How does login work?"}) ||
		task.Conversation[1].CreatorName != "agent" || task.Conversation[1].Message != "Through the auth middleware." {
		t.Errorf("conversation = %+v", task.Conversation)
	}
}

func TestParseStageExecutePayload_ChatModeDefaultsToReply(t *testing.T) {
	event := unmarshalEvent(t, `{"idRun": 42, "payload": {"idTask": 1, "stage": "chat", "attemptNo": 1, "runToken": "rdr_x", "contextBundle": {"chat": {"idConversation": 9, "mode": "something-new"}}}}`)
	task, err := parseStageExecutePayload(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.Chat == nil || task.Chat.Mode != ChatModeReply {
		t.Errorf("chat = %+v, want mode %s", task.Chat, ChatModeReply)
	}
}

func TestRenderPrompt_ChatContainsHistoryProjectsAndReadOnlyRules(t *testing.T) {
	conversation := []TaskMessage{
		{CreatorName: "user", Kind: "text", CreatedAt: "t1", Message: "first question"},
		{CreatorName: "agent", Kind: "text", CreatedAt: "t2", Message: "first answer"},
		{CreatorName: "user", Kind: "text", CreatedAt: "t3", Message: "second question"},
	}
	projects := []ChatProject{{IdProject: 1, Name: "Rurdesk", IsMain: true}, {IdProject: 3, Name: "Docs"}}
	for _, tt := range []struct {
		name    string
		chat    ChatContext
		want    []string
		notWant []string
	}{
		{
			name:    "reply",
			chat:    ChatContext{Mode: ChatModeReply, Projects: projects, HeadCommit: "abc123", WorktreeHead: "abc123"},
			want:    []string{"message_kind=chat_reply", "abc123", "asks you to create or draft a task", "message_kind=task_draft", "`# <title>`"},
			notWant: []string{"oldest messages were left out", "code was refreshed", "draft one task about the answer quoted below"},
		},
		{
			name:    "task_draft",
			chat:    ChatContext{Mode: ChatModeTaskDraft, Projects: projects},
			want:    []string{"message_kind=task_draft", "`# <title>`", "No repository"},
			notWant: []string{"message_kind=chat_reply", "asks you to create or draft a task", "quoted below"},
		},
		{
			name: "task_draft from an answer",
			chat: ChatContext{Mode: ChatModeTaskDraft, Projects: projects,
				DraftFromMessage: &ChatDraftFromMessage{IdMessage: 7, Body: "Quoted answer: add a retry button"}},
			want: []string{"draft one task about the answer quoted below", "use the rest of the conversation only as context",
				"Quoted answer: add a retry button", "message_kind=task_draft"},
			notWant: []string{"message_kind=chat_reply"},
		},
		{
			name: "truncated",
			chat: ChatContext{Mode: ChatModeReply, Projects: projects, IsHistoryTruncated: true},
			want: []string{"oldest messages were left out"},
		},
		{
			name: "code refreshed",
			chat: ChatContext{Mode: ChatModeReply, Projects: projects, HeadCommit: "old111", WorktreeHead: "new222"},
			want: []string{"code was refreshed", "old111", "new222"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			chat := tt.chat
			task := Task{IdTask: 77, IdRun: 500, Stage: StageChat, Conversation: conversation, Chat: &chat, Repo: AgentRunGitRepo{BaseBranch: "main"}}

			prompt, err := RenderPrompt(task)
			if err != nil {
				t.Fatalf("RenderPrompt: %v", err)
			}
			always := []string{
				"Rurdesk (project_id: 1) — main project", "Docs (project_id: 3)",
				"Read-only", "do not commit or push", "`Write`", "`Edit`",
				"complete_stage", "id_task=77",
			}
			for _, want := range append(always, tt.want...) {
				if !strings.Contains(prompt, want) {
					t.Errorf("prompt lacks %q\n%s", want, prompt)
				}
			}
			for _, notWant := range tt.notWant {
				if strings.Contains(prompt, notWant) {
					t.Errorf("prompt contains %q\n%s", notWant, prompt)
				}
			}
			first, second, third := strings.Index(prompt, "first question"), strings.Index(prompt, "first answer"), strings.Index(prompt, "second question")
			if first < 0 || first > second || second > third {
				t.Errorf("history is not oldest first\n%s", prompt)
			}
		})
	}
}

func TestRenderPrompt_ChatNeedsItsChatContext(t *testing.T) {
	if _, err := RenderPrompt(Task{IdTask: 77, Stage: StageChat}); err == nil {
		t.Error("a chat turn without its chat context rendered a prompt")
	}
}

func TestTaskSessionUUID_ChatTurnsNeverCollide(t *testing.T) {
	seen := map[string]int64{}
	for idTask := int64(1); idTask <= 15; idTask++ {
		got := TaskSessionUUID(Task{IdRun: 500, IdTask: idTask, Stage: StageChat, AttemptNo: int(idTask)})
		if !uuidV4Pattern.MatchString(got) {
			t.Errorf("chat session id %q is not a v4 UUID", got)
		}
		if previous, ok := seen[got]; ok {
			t.Errorf("chat turns %d and %d share the session id %s", previous, idTask, got)
		}
		seen[got] = idTask
	}
	issue := Task{IdRun: 500, IdTask: 3, Stage: StageDesign, AttemptNo: 2}
	if got := TaskSessionUUID(issue); got != SessionUUID(500, StageDesign, 2) {
		t.Errorf("issue session id = %q, want SessionUUID unchanged", got)
	}
	if _, ok := seen[SessionUUID(500, StageDesign, 2)]; ok {
		t.Error("a chat session id collides with an issue stage session id")
	}
}

func TestChatConstantsMirrorAPI(t *testing.T) {
	for _, c := range []struct{ got, want string }{
		{StageChat, "chat"},
		{ChatModeReply, "reply"},
		{ChatModeTaskDraft, "task_draft"},
		{ChatMessageKindReply, "chat_reply"},
		{ChatMessageKindTaskDraft, "task_draft"},
	} {
		if c.got != c.want {
			t.Errorf("chat constant drift: got %q want %q", c.got, c.want)
		}
	}
}

func TestEnsureRepo_NoOtherCredentialHelperGetsTheToken(t *testing.T) {
	tmp := newGitEnv(t)
	originPath, _ := seedOrigin(t, tmp, "main")
	git(t, originPath, "config", "http.receivepack", "true")
	repo := testRepo(authGitServer(t, tmp, testGitToken)+"/origin.git", "example.com", "main")
	// Set after the server's guard: this file-backed store is the one helper
	// the test wants in effect.
	storePath := filepath.Join(tmp, "stored-credentials")
	globalConfig := filepath.Join(tmp, "global.gitconfig")
	if err := os.WriteFile(globalConfig, []byte("[credential]\n\thelper = store --file "+storePath+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", globalConfig)
	workspaceBase := filepath.Join(tmp, "ws")

	repoPath := ensureRepo(t, workspaceBase, repo)
	if _, err := CreateDetachedWorktree(repoPath, testGitToken, "main", 7); err != nil {
		t.Fatalf("CreateDetachedWorktree: %v", err)
	}
	ensureRepo(t, workspaceBase, repo)

	if stored, err := os.ReadFile(storePath); err == nil && strings.Contains(string(stored), testGitToken) {
		t.Error("a credential helper of the global config stored the git token")
	}
	assertNoFileHolds(t, workspaceBase, testGitToken)
}

// agentEditsWorktree leaves what a chat agent could: a commit on a branch of
// its own, an uncommitted change and an ignored-or-not untracked file.
func agentEditsWorktree(t *testing.T, worktreePath string) {
	t.Helper()
	git(t, worktreePath, "checkout", "-b", "agent-own")
	if err := os.WriteFile(filepath.Join(worktreePath, "README.md"), []byte("committed by the agent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, worktreePath, "commit", "-am", "agent commit")
	if err := os.WriteFile(filepath.Join(worktreePath, "README.md"), []byte("uncommitted by the agent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktreePath, "untracked.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunStage_Chat_ReuseResetsAgentChangesToRecordedHead(t *testing.T) {
	agent := &recordingAgent{}
	orchestrator, tracker, repo, seedPath := chatTestOrchestrator(t, agent)
	orchestrator.runStage(chatTask(repo, 100))
	worktreePath := runWorktreePath(t, orchestrator, repo, 500)
	recordedHead := headSha(t, worktreePath)
	agentEditsWorktree(t, worktreePath)
	pushCommit(t, seedPath, "later.txt")
	if err := os.Rename(repo.CloneUrl, repo.CloneUrl+".gone"); err != nil {
		t.Fatal(err)
	}

	orchestrator.runStage(chatTaskAt(repo, 101, recordedHead))

	if len(agent.ran()) != 2 {
		t.Fatalf("agent ran %d times, want 2 (complete: %v)", len(agent.ran()), tracker.body("/agent/task/101/complete"))
	}
	if got := headSha(t, worktreePath); got != recordedHead {
		t.Errorf("worktree HEAD = %s, want the recorded head %s", got, recordedHead)
	}
	if branch, err := branchOfWorktree(worktreePath); err != nil || branch != "" {
		t.Errorf("worktree is on branch %q (err %v), want a detached HEAD", branch, err)
	}
	assertFile(t, filepath.Join(worktreePath, "README.md"), "origin\n")
	if !isMissing(filepath.Join(worktreePath, "untracked.txt")) {
		t.Error("an untracked file of the agent survived the reset")
	}
	if body := tracker.body("/agent/task/101/stats"); body == nil || body["headCommit"] != recordedHead {
		t.Errorf("stats = %v, want headCommit %s", body, recordedHead)
	}
}

func TestRunStage_Chat_ReuseWithoutUsableRecordedHeadRecreates(t *testing.T) {
	for _, tt := range []struct {
		name       string
		headCommit string
	}{
		{"no recorded head", ""},
		{"recorded head unknown to the clone", "0123456789abcdef0123456789abcdef01234567"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			agent := &recordingAgent{}
			orchestrator, tracker, repo, seedPath := chatTestOrchestrator(t, agent)
			orchestrator.runStage(chatTask(repo, 100))
			worktreePath := runWorktreePath(t, orchestrator, repo, 500)
			agentEditsWorktree(t, worktreePath)
			newHead := pushCommit(t, seedPath, "later.txt")

			orchestrator.runStage(chatTaskAt(repo, 101, tt.headCommit))

			if got := headSha(t, worktreePath); got != newHead {
				t.Errorf("worktree HEAD = %s, want it recreated at origin/main %s", got, newHead)
			}
			assertFile(t, filepath.Join(worktreePath, "README.md"), "origin\n")
			if !isMissing(filepath.Join(worktreePath, "untracked.txt")) {
				t.Error("an untracked file of the agent survived")
			}
			if body := tracker.body("/agent/task/101/stats"); body == nil || body["headCommit"] != newHead {
				t.Errorf("stats = %v, want headCommit %s", body, newHead)
			}
		})
	}
}

func TestRunStage_Chat_EmptyOriginRunsInScratchDirWithoutPush(t *testing.T) {
	tmp := newGitEnv(t)
	originPath := filepath.Join(tmp, "empty.git")
	git(t, tmp, "init", "--bare", "-b", "main", originPath)
	tracker, trackerURL := newChatTracker(t)
	cfg := &Config{TrackerAPIUrl: trackerURL, WebhookSecret: testWebhookSecret, WorkspaceBase: filepath.Join(tmp, "ws")}
	agent := &workspaceRecordingAgent{workspace: make(chan string, 1), entries: make(chan int, 1)}
	orchestrator := NewOrchestrator(cfg, agent, NewTrackerClient(cfg), NewState())

	orchestrator.runStage(chatTask(testRepo(originPath, "example.com", "main"), 100))

	select {
	case workspace := <-agent.workspace:
		if entries := <-agent.entries; entries != 0 {
			t.Errorf("the scratch directory held %d entries, want none", entries)
		}
		if strings.HasPrefix(workspace, cfg.WorkspaceBase) || !isMissing(workspace) {
			t.Errorf("the turn ran in %s, want a removed scratch directory outside the workspace", workspace)
		}
	default:
		t.Fatalf("the agent never ran: a chat on an empty repo is a chat without code (complete: %v)", tracker.body("/agent/task/100/complete"))
	}
	if refs, _ := runGitQuietOutput(originPath, "for-each-ref"); strings.TrimSpace(refs) != "" {
		t.Errorf("a chat turn pushed to the empty origin: %q", refs)
	}
	if body := tracker.body("/agent/task/100/stats"); body != nil && body["headCommit"] != nil {
		t.Errorf("a chat on an empty repo reported headCommit %v", body["headCommit"])
	}
}

func TestRenderPrompt_ChatClaudeAllowsNoShell(t *testing.T) {
	prompt, err := RenderPrompt(Task{IdTask: 77, Stage: StageChat, Chat: &ChatContext{Mode: ChatModeReply}, Vocab: ToolVocabClaudeCode})
	if err != nil {
		t.Fatalf("RenderPrompt: %v", err)
	}
	allowed, _, _ := strings.Cut(prompt[strings.Index(prompt, "ALLOWED tools:"):], "\n")
	if strings.Contains(allowed, "Bash") {
		t.Errorf("ALLOWED offers the shell that FORBIDDEN takes away: %q", allowed)
	}
	if !strings.Contains(allowed, "`Glob`") || !strings.Contains(allowed, "`Read`") || !strings.Contains(allowed, "`Grep`") {
		t.Errorf("ALLOWED lacks the read tools: %q", allowed)
	}
}
