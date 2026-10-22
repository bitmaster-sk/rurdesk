package common

import "testing"

func TestSecretSet_RedactJson(t *testing.T) {
	secrets := NewSecretSet()
	secrets.Add("ghp_secret_token_1")
	secrets.Add("short")

	got := string(secrets.RedactJson([]byte(`{"text":"push https://x-token:ghp_secret_token_1@host failed, short"}`)))

	if want := `{"text":"push https://x-token:***@host failed, short"}`; got != want {
		t.Errorf("RedactJson = %s, want %s", got, want)
	}
}

func TestSecretSet_KeepsASecretWhileAnotherStageStillUsesIt(t *testing.T) {
	secrets := NewSecretSet()
	secrets.Add("ghp_shared_token_1")
	secrets.Add("ghp_shared_token_1")

	secrets.Remove("ghp_shared_token_1")
	if got := string(secrets.RedactJson([]byte(`"ghp_shared_token_1"`))); got != `"***"` {
		t.Errorf("after one of two stages ended: %s, want the token still masked", got)
	}

	secrets.Remove("ghp_shared_token_1")
	if got := string(secrets.RedactJson([]byte(`"ghp_shared_token_1"`))); got != `"ghp_shared_token_1"` {
		t.Errorf("after both stages ended: %s, want the token forgotten", got)
	}
}
