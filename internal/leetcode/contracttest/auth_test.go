package contracttest

import (
	"strings"
	"testing"
)

// When neither env vars nor a creds file are present, the resolver must
// hand back an instruction string that names the populate command. This
// exact text is what LoadTestCreds passes to t.Fatal — it's the only
// thing a fresh clone running `go test -tags integration ./...` sees
// when the live contract can't run.
func TestResolveTestCreds_MissingEverywhere_ReturnsSetupPointer(t *testing.T) {
	t.Setenv("LEETCODE_TEST_SESSION", "")
	t.Setenv("LEETCODE_TEST_CSRF", "")
	// Point UserConfigDir at an empty temp dir so TestCredsPath resolves
	// somewhere with no creds file.
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	creds, msg := resolveTestCreds()
	if creds != nil {
		t.Fatalf("creds = %+v, want nil with no env/file", creds)
	}
	if !strings.Contains(msg, "leetcode-test-login") {
		t.Errorf("setup pointer missing the populate command name: %q", msg)
	}
}

func TestResolveTestCreds_EnvVarsPopulated_UsesEnv(t *testing.T) {
	t.Setenv("LEETCODE_TEST_SESSION", "env-sess")
	t.Setenv("LEETCODE_TEST_CSRF", "env-csrf")

	creds, msg := resolveTestCreds()
	if msg != "" {
		t.Errorf("msg = %q, want empty when env is populated", msg)
	}
	if creds == nil || creds.Session != "env-sess" || creds.CSRF != "env-csrf" {
		t.Errorf("creds = %+v, want env-sess/env-csrf", creds)
	}
}
