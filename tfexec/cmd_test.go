// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package tfexec

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/magosproject/terraform-exec/internal/version"
)

func TestMergeUserAgent(t *testing.T) {
	for i, c := range []struct {
		expected string
		uas      []string
	}{
		{"foo/1 bar/2", []string{"foo/1", "bar/2"}},
		{"foo/1 bar/2", []string{"foo/1 bar/2"}},
		{"foo/1 bar/2", []string{"", "foo/1", "bar/2"}},
		{"foo/1 bar/2", []string{"", "foo/1 bar/2"}},
		{"foo/1 bar/2", []string{"  ", "foo/1 bar/2"}},
		{"foo/1 bar/2", []string{"foo/1", "", "bar/2"}},
		{"foo/1 bar/2", []string{"foo/1", "   ", "bar/2"}},

		// comments
		{"foo/1 (bar/1 bar/2 bar/3) bar/2", []string{"foo/1 (bar/1 bar/2 bar/3)", "bar/2"}},
	} {
		t.Run(fmt.Sprintf("%d %s", i, c.expected), func(t *testing.T) {
			actual := mergeUserAgent(c.uas...)
			if c.expected != actual {
				t.Fatalf("expected %q, got %q", c.expected, actual)
			}
		})
	}
}

func defaultEnv() []string {
	return []string{
		"CHECKPOINT_DISABLE=",
		"TF_APPEND_USER_AGENT=HashiCorp-terraform-exec/" + version.ModuleVersion(),
		"TF_IN_AUTOMATION=1",
		"TF_LOG=",
		"TF_LOG_CORE=",
		"TF_LOG_PATH=",
		"TF_LOG_PROVIDER=",
	}
}

// stripOsEnv removes variables inherited from os.Environ from env. assertCmd
// calls this on the actual command env so that tests can assert on the
// library-managed vars only, without needing to enumerate every var that
// happens to be set on the host running the tests.
func stripOsEnv(env map[string]string) {
	libManaged := envMap(defaultEnv())
	for k := range env {
		if _, ok := libManaged[k]; !ok {
			delete(env, k)
		}
	}
}

// assertCmd asserts that a constructed exec.Cmd contains the expected args and environment variables.
// The command itself isn't executed; that is only done in E2E tests.
func assertCmd(t *testing.T, expectedArgs []string, expectedEnv map[string]string, actual *exec.Cmd) {
	t.Helper()

	// check args (skip path)
	actualArgs := actual.Args[1:]

	if len(expectedArgs) != len(actualArgs) {
		t.Fatalf("args mismatch\n\nexpected:\n%v\n\ngot:\n%v", strings.Join(expectedArgs, " "), strings.Join(actualArgs, " "))
	}
	for i := range expectedArgs {
		if expectedArgs[i] != actualArgs[i] {
			t.Fatalf("args mismatch, expected %q, got %q\n\nfull expected:\n%v\n\nfull actual:\n%v", expectedArgs[i], actualArgs[i], strings.Join(expectedArgs, " "), strings.Join(actualArgs, " "))
		}
	}

	// check environment
	expectedEnv = envMap(append(defaultEnv(), envSlice(expectedEnv)...))
	actualEnv := envMap(actual.Env)

	if len(actualEnv) != len(actual.Env) {
		t.Fatalf("duplication in actual env, unable to assert: %v", actual.Env)
	}

	// ignore tempdir related env vars from comparison
	for _, k := range []string{"TMPDIR", "TMP", "TEMP", "USERPROFILE"} {
		if _, ok := expectedEnv[k]; ok {
			t.Logf("ignoring env var %q", k)
			delete(expectedEnv, k)
		}

		if _, ok := actualEnv[k]; ok {
			t.Logf("ignoring env var %q", k)
			delete(actualEnv, k)
		}
	}

	// strip vars that came from os.Environ: they are not what command-level
	// tests are asserting on and vary between machines
	stripOsEnv(actualEnv)

	// compare against raw slice len incase of duplication or something
	if len(expectedEnv) != len(actualEnv) {
		t.Fatalf("env mismatch\n\nexpected:\n%v\n\ngot:\n%v", envSlice(expectedEnv), actual.Env)
	}

	for k, ev := range expectedEnv {
		av, ok := actualEnv[k]
		if !ok {
			t.Fatalf("env mismatch, missing %q\n\nfull expected:\n%v\n\nfull actual:\n%v", k, envSlice(expectedEnv), envSlice(actualEnv))
		}
		if ev != av {
			t.Fatalf("env mismatch, expected %q, got %q\n\nfull expected:\n%v\n\nfull actual:\n%v", ev, av, envSlice(expectedEnv), envSlice(actualEnv))
		}
	}
}

func TestBuildEnvSetEnvDoesNotStripOsEnv(t *testing.T) {
	td := t.TempDir()
	tf, err := NewTerraform(td, "echo")
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", "/test/home")
	t.Setenv("AWS_REGION", "us-east-1")

	if err := tf.SetEnv(map[string]string{"TF_PLUGIN_CACHE_DIR": "/cache"}); err != nil {
		t.Fatal(err)
	}

	env := envMap(tf.buildTerraformCmd(t.Context(), nil).Env)

	for k, want := range map[string]string{
		"HOME":                "/test/home",
		"AWS_REGION":          "us-east-1",
		"TF_PLUGIN_CACHE_DIR": "/cache",
	} {
		if got := env[k]; got != want {
			t.Fatalf("expected %q=%q in cmd env after SetEnv, got %q", k, want, got)
		}
	}
}
