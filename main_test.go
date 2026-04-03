package main

import (
	"errors"
	"os"
	"testing"
)

func TestMain_HelpPath(t *testing.T) {
	oldArgs := os.Args
	os.Args = []string{"sim-cli", "--help"}
	t.Cleanup(func() {
		os.Args = oldArgs
	})

	main()
}

func TestMain_DoesNotExitOnSuccess(t *testing.T) {
	oldExecute := execute
	oldOsExit := osExit
	t.Cleanup(func() {
		execute = oldExecute
		osExit = oldOsExit
	})

	execute = func() error { return nil }
	osExit = func(code int) {
		t.Fatalf("osExit should not be called on success, got code %d", code)
	}

	main()
}

func TestMain_ExitsWithCode1OnError(t *testing.T) {
	oldExecute := execute
	oldOsExit := osExit
	t.Cleanup(func() {
		execute = oldExecute
		osExit = oldOsExit
	})

	execute = func() error { return errors.New("boom") }

	called := false
	osExit = func(code int) {
		called = true
		if code != 1 {
			t.Fatalf("osExit code = %d, want 1", code)
		}
	}

	main()

	if !called {
		t.Fatal("expected osExit to be called")
	}
}
