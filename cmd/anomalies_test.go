package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestMustMarkRequired_Success(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("duration", "", "")

	mustMarkRequired(cmd, "duration")

	f := cmd.Flags().Lookup("duration")
	if f == nil {
		t.Fatal("duration flag should exist")
	}
	if f.Annotations == nil || len(f.Annotations[cobra.BashCompOneRequiredFlag]) == 0 {
		t.Fatal("duration flag should be marked as required")
	}
}

func TestMustMarkRequired_ErrorTriggersExit(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	called := false
	code := 0

	prevExit := exitProcess
	exitProcess = func(c int) {
		called = true
		code = c
	}
	t.Cleanup(func() {
		exitProcess = prevExit
	})

	mustMarkRequired(cmd, "missing-flag")

	if !called {
		t.Fatal("expected exit function to be called")
	}
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}
