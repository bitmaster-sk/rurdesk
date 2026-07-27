package test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/adminops"
	"github.com/bitmaster-sk/rurdesk/api/internal/githost"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/suite"
)

type RotateGitTokensSuite struct {
	suite.Suite
	App       *issue.Application
	Token     string
	ProjectID int64
	oldKey    []byte
	newKey    []byte
}

func (s *RotateGitTokensSuite) SetupSuite() {
	os.Setenv("GIT_INTEGRATION_ENCRYPTION_KEY", "QUJDREVGR0hJSktMTU5PUFFSU1RVVldYWVoxMjM0NTY=")
	githost.ResetEncryptionKey()

	s.App = Setup(s.T())
	s.Token = Token(s.T(), s.App)

	key, err := githost.LoadEncryptionKey()
	s.Require().NoError(err)
	s.oldKey = key
	s.newKey = []byte("0123456789abcdef0123456789abcdef") // 32 bytes

	prjRes := Request(s.T(), s.App, "POST", "/api/private/project",
		`{"name":"rotate-key-test","color":"#123456"}`, s.Token)
	s.Require().Equal(http.StatusOK, prjRes.StatusCode)
	var prj model.Project
	s.Require().NoError(json.NewDecoder(prjRes.Body).Decode(&prj))
	s.ProjectID = prj.IdProject
}

func (s *RotateGitTokensSuite) TearDownSuite() {
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM projects.project WHERE id_project = $1", s.ProjectID)
}

func (s *RotateGitTokensSuite) SetupTest() {
	ctx := context.Background()
	_, err := s.App.Pool.Exec(ctx, `
		UPDATE issues.issue SET id_git_integration = NULL, mr_id = NULL
		WHERE id_git_integration IS NOT NULL`)
	s.Require().NoError(err)
	_, err = s.App.Pool.Exec(ctx, "DELETE FROM projects.git_integration")
	s.Require().NoError(err)
}

func (s *RotateGitTokensSuite) seedIntegration(repoPath, plainToken string) int64 {
	ciphertext, nonce, err := githost.Encrypt(s.oldKey, []byte(plainToken))
	s.Require().NoError(err)

	var id int64
	err = s.App.Pool.QueryRow(context.Background(), `
		INSERT INTO projects.git_integration
			(id_project, name, host_type, base_url, repo_path, access_token_enc, token_nonce)
		VALUES ($1, $2, 'github', 'https://github.com', $3, $4, $5)
		RETURNING id_git_integration`,
		s.ProjectID, "rot-"+repoPath, repoPath, ciphertext, nonce,
	).Scan(&id)
	s.Require().NoError(err)
	return id
}

func (s *RotateGitTokensSuite) rawToken(id int64) (ciphertext, nonce []byte) {
	err := s.App.Pool.QueryRow(context.Background(),
		"SELECT access_token_enc, token_nonce FROM projects.git_integration WHERE id_git_integration = $1",
		id,
	).Scan(&ciphertext, &nonce)
	s.Require().NoError(err)
	return ciphertext, nonce
}

func (s *RotateGitTokensSuite) Test_Rotate_ReEncryptsEveryToken() {
	idA := s.seedIntegration("org/repo-a", "ghp_token_a")
	idB := s.seedIntegration("org/repo-b", "ghp_token_b")

	n, err := adminops.RotateGitTokens(context.Background(), s.App.Pool, s.oldKey, s.newKey)
	s.Require().NoError(err)
	s.Equal(2, n, "both integrations must be reported as rotated")

	for id, want := range map[int64]string{idA: "ghp_token_a", idB: "ghp_token_b"} {
		ciphertext, nonce := s.rawToken(id)

		plain, decErr := githost.Decrypt(s.newKey, nonce, ciphertext)
		s.Require().NoError(decErr, "token must decrypt under the new key after rotation")
		s.Equal(want, string(plain), "rotation must preserve the token value")

		_, oldErr := githost.Decrypt(s.oldKey, nonce, ciphertext)
		s.Error(oldErr, "the old key must no longer decrypt a rotated token")
	}
}

func (s *RotateGitTokensSuite) Test_Rotate_WrongOldKey_LeavesDataUntouched() {
	id := s.seedIntegration("org/repo-wrong-key", "ghp_untouched")
	beforeCiphertext, beforeNonce := s.rawToken(id)

	wrongOldKey := []byte("ffffffffffffffffffffffffffffffff") // 32 bytes, not the real key
	n, err := adminops.RotateGitTokens(context.Background(), s.App.Pool, wrongOldKey, s.newKey)
	s.Error(err, "a wrong old key must fail the rotation")
	s.Zero(n)

	afterCiphertext, afterNonce := s.rawToken(id)
	s.Equal(beforeCiphertext, afterCiphertext, "ciphertext must be byte-identical after a failed rotation")
	s.Equal(beforeNonce, afterNonce, "nonce must be byte-identical after a failed rotation")

	plain, err := githost.Decrypt(s.oldKey, afterNonce, afterCiphertext)
	s.Require().NoError(err, "the original key must still decrypt the untouched token")
	s.Equal("ghp_untouched", string(plain))
}

func (s *RotateGitTokensSuite) Test_Rotate_PartialFailure_RollsBackEarlierRows() {
	idGood := s.seedIntegration("org/repo-good", "ghp_good")
	beforeCiphertext, _ := s.rawToken(idGood)

	foreignKey := []byte("eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee")
	badCiphertext, badNonce, err := githost.Encrypt(foreignKey, []byte("ghp_bad"))
	s.Require().NoError(err)
	_, err = s.App.Pool.Exec(context.Background(), `
		INSERT INTO projects.git_integration
			(id_project, name, host_type, base_url, repo_path, access_token_enc, token_nonce)
		VALUES ($1, 'rot-bad', 'github', 'https://github.com', 'org/repo-bad', $2, $3)`,
		s.ProjectID, badCiphertext, badNonce)
	s.Require().NoError(err)

	n, err := adminops.RotateGitTokens(context.Background(), s.App.Pool, s.oldKey, s.newKey)
	s.Error(err)
	s.Zero(n)

	afterCiphertext, afterNonce := s.rawToken(idGood)
	s.Equal(beforeCiphertext, afterCiphertext, "the decryptable row must not stay rotated after the batch failed")
	plain, err := githost.Decrypt(s.oldKey, afterNonce, afterCiphertext)
	s.Require().NoError(err)
	s.Equal("ghp_good", string(plain))
}

// A token replaced while the rotation is in flight must survive it. Reading
// outside the transaction re-encrypts the stale plaintext over the newer token,
// which the operator only discovers when the token stops working.
func (s *RotateGitTokensSuite) Test_Rotate_DoesNotClobberConcurrentTokenWrite() {
	ctx := context.Background()
	id := s.seedIntegration("org/repo-race", "ghp_v1")

	writer, err := s.App.Pool.Begin(ctx)
	s.Require().NoError(err)
	defer func() { _ = writer.Rollback(ctx) }()

	var lockedID int64
	s.Require().NoError(writer.QueryRow(ctx,
		"SELECT id_git_integration FROM projects.git_integration WHERE id_git_integration = $1 FOR UPDATE",
		id,
	).Scan(&lockedID))

	rotated := make(chan error, 1)
	go func() {
		_, rotateErr := adminops.RotateGitTokens(ctx, s.App.Pool, s.oldKey, s.newKey)
		rotated <- rotateErr
	}()

	time.Sleep(500 * time.Millisecond)

	newCiphertext, newNonce, err := githost.Encrypt(s.oldKey, []byte("ghp_v2"))
	s.Require().NoError(err)
	_, err = writer.Exec(ctx, `
		UPDATE projects.git_integration SET access_token_enc = $1, token_nonce = $2
		WHERE id_git_integration = $3`, newCiphertext, newNonce, id)
	s.Require().NoError(err)
	s.Require().NoError(writer.Commit(ctx))

	select {
	case rotateErr := <-rotated:
		s.Require().NoError(rotateErr)
	case <-time.After(15 * time.Second):
		s.FailNow("rotation did not finish after the concurrent writer committed")
	}

	ciphertext, nonce := s.rawToken(id)
	plain, err := githost.Decrypt(s.newKey, nonce, ciphertext)
	s.Require().NoError(err, "the stored token must be readable with the new key")
	s.Equal("ghp_v2", string(plain),
		"the rotation must re-encrypt the token as committed by the concurrent writer, not the value it read earlier")
}

func Test_RunRotateGitTokensSuite(t *testing.T) {
	suite.Run(t, new(RotateGitTokensSuite))
}
