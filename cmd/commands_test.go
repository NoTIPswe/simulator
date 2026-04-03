package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/NoTIPswe/notip-simulator-cli/internal/client"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// TestMain disables styled/color output before any test runs.
//
// Command code uses startSpinner(), which returns a no-op spinner when
// pterm.RawOutput is enabled (set by DisableStyling). This avoids creating
// PTerm spinner goroutines in tests and prevents the known -race issue in
// pterm v0.12.79.
func TestMain(m *testing.M) {
	pterm.DisableOutput()
	pterm.DisableStyling()
	pterm.DisableColor()
	os.Exit(m.Run())
}

// ── test constants ────────────────────────────────────────────────────────────

const (
	testGatewayUUID      = "uuid-1"
	cmdNetDegradation    = "network-degradation"
	fmtUnexpectedPath    = "unexpected path: %s"
	fmtUnexpectedRequest = "unexpected request: %s %s"
	testFlagFactoryID    = "--factory-id"
	testFlagFactoryKey   = "--factory-key"
	testFlagDuration     = "--duration"
	bodyNotFound         = "not found"
	errExpected404       = "expected error on 404"
	fmtPipeErr           = "pipe: %v"
	fmtWriteErr          = "write: %v"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// resetAllFlags walks the entire command tree and resets every flag to its
// declared default value, clearing the Changed bit. This is necessary because
// Cobra/pflags do not reset flag state between Execute() calls in the same
// process, which would otherwise cause test pollution when the full suite runs.
func resetAllFlags(c *cobra.Command) {
	c.Flags().VisitAll(func(f *pflag.Flag) {
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	})
	for _, child := range c.Commands() {
		resetAllFlags(child)
	}
}

// runCmd resets all flags, sets args on the root command, and executes it.
func runCmd(args ...string) error {
	resetAllFlags(rootCmd)
	rootCmd.SetArgs(args)
	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})
	return rootCmd.Execute()
}

// newMockServer spins up an httptest server, sets simulatorURL, and registers cleanup.
func newMockServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(func() {
		srv.Close()
		simulatorURL = "http://simulator:8090" // restore default
	})
	simulatorURL = srv.URL
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// ── Command tree ──────────────────────────────────────────────────────────────

func TestCommandTree(t *testing.T) {
	tests := []struct {
		parent string
		child  string
	}{
		{"gateways", "list"},
		{"gateways", "get"},
		{"gateways", "create"},
		{"gateways", "bulk"},
		{"gateways", "start"},
		{"gateways", "stop"},
		{"gateways", "delete"},
		{"sensors", "add"},
		{"sensors", "list"},
		{"sensors", "delete"},
		{"anomalies", "disconnect"},
		{"anomalies", cmdNetDegradation},
		{"anomalies", "outlier"},
	}

	for _, tt := range tests {
		t.Run(tt.parent+"/"+tt.child, func(t *testing.T) {
			parent, _, err := rootCmd.Find([]string{tt.parent})
			if err != nil || parent == nil {
				t.Fatalf("parent command %q not found", tt.parent)
			}
			child, _, err := parent.Find([]string{tt.child})
			if err != nil || child == nil || child.Use == "" {
				t.Errorf("child command %q not found under %q", tt.child, tt.parent)
			}
		})
	}
}

// ── Required-flag validation ──────────────────────────────────────────────────

func TestGatewaysCreate_MissingRequiredFlags(t *testing.T) {
	// factory-id, factory-key, serial are all required
	if err := runCmd("gateways", "create"); err == nil {
		t.Error("expected error when required flags are missing")
	}
}

func TestGatewaysBulk_MissingCount(t *testing.T) {
	if err := runCmd("gateways", "bulk", testFlagFactoryID, "f", testFlagFactoryKey, "k"); err == nil {
		t.Error("expected error when --count is missing")
	}
}

func TestSensorsAdd_MissingFlags(t *testing.T) {
	if err := runCmd("sensors", "add", "5"); err == nil {
		t.Error("expected error when sensor flags are missing")
	}
}

func TestAnomaliesDisconnect_MissingDuration(t *testing.T) {
	if err := runCmd("anomalies", "disconnect", testGatewayUUID); err == nil {
		t.Error("expected error when --duration is missing")
	}
}

func TestAnomaliesNetworkDegradation_MissingDuration(t *testing.T) {
	if err := runCmd("anomalies", cmdNetDegradation, testGatewayUUID); err == nil {
		t.Error("expected error when --duration is missing")
	}
}

// ── Argument-count validation ─────────────────────────────────────────────────

func TestGatewaysGet_NoArgs(t *testing.T) {
	if err := runCmd("gateways", "get"); err == nil {
		t.Error("expected error when uuid arg is missing")
	}
}

func TestGatewaysDelete_NoArgs(t *testing.T) {
	if err := runCmd("gateways", "delete"); err == nil {
		t.Error("expected error when uuid arg is missing")
	}
}

func TestSensorsAdd_NonNumericID(t *testing.T) {
	newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Should never reach the server — arg parsing fails first.
		t.Error("server should not have been called")
	})
	if err := runCmd("sensors", "add", "not-a-number",
		"--type", "temperature", "--min", "0", "--max", "100", "--algorithm", "constant"); err == nil {
		t.Error("expected error for non-numeric gateway ID")
	}
}

func TestSensorsDelete_NonNumericID(t *testing.T) {
	if err := runCmd("sensors", "delete", "abc"); err == nil {
		t.Error("expected error for non-numeric sensor ID")
	}
}

func TestAnomaliesOutlier_NonNumericID(t *testing.T) {
	if err := runCmd("anomalies", "outlier", "not-a-number"); err == nil {
		t.Error("expected error for non-numeric sensor ID")
	}
}

// ── Integration: full execution against mock server ───────────────────────────

func TestGatewaysList_Integration(t *testing.T) {
	gateways := []map[string]any{
		{"id": 1, "managementGatewayId": testGatewayUUID, "status": "online", "model": "X", "serialNumber": "SN1", "sendFrequencyMs": 1000, "tenantId": "t1"},
	}
	newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/sim/gateways" {
			t.Errorf(fmtUnexpectedRequest, r.Method, r.URL.Path)
		}
		writeJSON(w, http.StatusOK, gateways)
	})

	if err := runCmd("gateways", "list"); err != nil {
		t.Fatalf("gateways list failed: %v", err)
	}
}

func TestGatewaysList_ServerError(t *testing.T) {
	newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	if err := runCmd("gateways", "list"); err == nil {
		t.Error("expected error when server returns 500")
	}
}

func TestGatewaysStart_Integration(t *testing.T) {
	newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sim/gateways/uuid-1/start" {
			t.Errorf(fmtUnexpectedPath, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	if err := runCmd("gateways", "start", testGatewayUUID); err != nil {
		t.Fatalf("gateways start failed: %v", err)
	}
}

func TestGatewaysStop_Integration(t *testing.T) {
	newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sim/gateways/uuid-1/stop" {
			t.Errorf(fmtUnexpectedPath, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	if err := runCmd("gateways", "stop", testGatewayUUID); err != nil {
		t.Fatalf("gateways stop failed: %v", err)
	}
}

func TestGatewaysDelete_Integration(t *testing.T) {
	newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/sim/gateways/uuid-1" {
			t.Errorf(fmtUnexpectedRequest, r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	if err := runCmd("gateways", "delete", testGatewayUUID); err != nil {
		t.Fatalf("gateways delete failed: %v", err)
	}
}

func TestSensorsList_Integration(t *testing.T) {
	sensors := []map[string]any{
		{"id": 1, "gatewayId": 5, "sensorId": "s-uuid-1", "type": "temperature", "minRange": 0, "maxRange": 100, "algorithm": "sine_wave"},
	}
	newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sim/gateways/5/sensors" {
			t.Errorf(fmtUnexpectedPath, r.URL.Path)
		}
		writeJSON(w, http.StatusOK, sensors)
	})
	if err := runCmd("sensors", "list", "5"); err != nil {
		t.Fatalf("sensors list failed: %v", err)
	}
}

func TestSensorsDelete_Integration(t *testing.T) {
	newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/sim/sensors/99" {
			t.Errorf(fmtUnexpectedRequest, r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	if err := runCmd("sensors", "delete", "99"); err != nil {
		t.Fatalf("sensors delete failed: %v", err)
	}
}

func TestAnomaliesDisconnect_Integration(t *testing.T) {
	newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sim/gateways/uuid-1/anomaly/disconnect" {
			t.Errorf(fmtUnexpectedPath, r.URL.Path)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["duration_seconds"] != float64(3) {
			t.Errorf("duration_seconds = %v, want 3", body["duration_seconds"])
		}
		w.WriteHeader(http.StatusNoContent)
	})
	if err := runCmd("anomalies", "disconnect", testGatewayUUID, testFlagDuration, "3"); err != nil {
		t.Fatalf("anomalies disconnect failed: %v", err)
	}
}

func TestAnomaliesNetworkDegradation_Integration(t *testing.T) {
	newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sim/gateways/uuid-1/anomaly/network-degradation" {
			t.Errorf(fmtUnexpectedPath, r.URL.Path)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["duration_seconds"] != float64(10) {
			t.Errorf("duration_seconds = %v, want 10", body["duration_seconds"])
		}
		if body["packet_loss_pct"] != 0.3 {
			t.Errorf("packet_loss_pct = %v, want 0.3", body["packet_loss_pct"])
		}
		w.WriteHeader(http.StatusNoContent)
	})
	if err := runCmd("anomalies", cmdNetDegradation, testGatewayUUID, testFlagDuration, "10", "--packet-loss", "0.3"); err != nil {
		t.Fatalf("anomalies network-degradation failed: %v", err)
	}
}

func TestAnomaliesOutlier_Integration(t *testing.T) {
	newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sim/sensors/42/anomaly/outlier" {
			t.Errorf(fmtUnexpectedPath, r.URL.Path)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["value"] != 999.9 {
			t.Errorf("value = %v, want 999.9", body["value"])
		}
		w.WriteHeader(http.StatusNoContent)
	})
	if err := runCmd("anomalies", "outlier", "42", "--value", "999.9"); err != nil {
		t.Fatalf("anomalies outlier failed: %v", err)
	}
}

// ── Error paths (spinner.Fail + return err) ───────────────────────────────────

func TestGatewaysList_EmptyResult(t *testing.T) {
	newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, []any{})
	})
	if err := runCmd("gateways", "list"); err != nil {
		t.Fatalf("gateways list empty failed: %v", err)
	}
}

func TestGatewaysGet_ServerError(t *testing.T) {
	newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, bodyNotFound, http.StatusNotFound)
	})
	if err := runCmd("gateways", "get", testGatewayUUID); err == nil {
		t.Error(errExpected404)
	}
}

func TestGatewaysCreate_ServerError(t *testing.T) {
	newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	})
	err := runCmd("gateways", "create", testFlagFactoryID, "f", testFlagFactoryKey, "k", "--serial", "SN")
	if err == nil {
		t.Error("expected error when server returns 400")
	}
}

func TestGatewaysBulk_ServerError(t *testing.T) {
	newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	})
	err := runCmd("gateways", "bulk", "--count", "2", testFlagFactoryID, "f", testFlagFactoryKey, "k")
	if err == nil {
		t.Error("expected error when bulk server returns 500")
	}
}

func TestGatewaysBulk_PartialErrors(t *testing.T) {
	newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusMultiStatus, map[string]any{
			"gateways": []any{map[string]any{"id": 1}},
			"errors":   []any{"", "factory key mismatch"},
		})
	})
	err := runCmd("gateways", "bulk", "--count", "2", testFlagFactoryID, "f", testFlagFactoryKey, "k")
	if err != nil {
		t.Fatalf("bulk partial error should succeed at cmd level: %v", err)
	}
}

func TestGatewaysStart_ServerError(t *testing.T) {
	newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "conflict", http.StatusConflict)
	})
	if err := runCmd("gateways", "start", testGatewayUUID); err == nil {
		t.Error("expected error on 409")
	}
}

func TestGatewaysStop_ServerError(t *testing.T) {
	newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, bodyNotFound, http.StatusNotFound)
	})
	if err := runCmd("gateways", "stop", testGatewayUUID); err == nil {
		t.Error(errExpected404)
	}
}

func TestGatewaysDelete_ServerError(t *testing.T) {
	newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, bodyNotFound, http.StatusNotFound)
	})
	if err := runCmd("gateways", "delete", testGatewayUUID); err == nil {
		t.Error(errExpected404)
	}
}

func TestSensorsAdd_ServerError(t *testing.T) {
	newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, bodyNotFound, http.StatusNotFound)
	})
	err := runCmd("sensors", "add", "5", "--type", "temperature", "--min", "0", "--max", "100", "--algorithm", "constant")
	if err == nil {
		t.Error(errExpected404)
	}
}

func TestSensorsList_EmptyResult(t *testing.T) {
	newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, []any{})
	})
	if err := runCmd("sensors", "list", "5"); err != nil {
		t.Fatalf("sensors list empty failed: %v", err)
	}
}

func TestSensorsList_ServerError(t *testing.T) {
	newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, bodyNotFound, http.StatusNotFound)
	})
	if err := runCmd("sensors", "list", "5"); err == nil {
		t.Error(errExpected404)
	}
}

func TestSensorsDelete_ServerError(t *testing.T) {
	newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, bodyNotFound, http.StatusNotFound)
	})
	if err := runCmd("sensors", "delete", "99"); err == nil {
		t.Error(errExpected404)
	}
}

func TestAnomaliesDisconnect_ServerError(t *testing.T) {
	newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, bodyNotFound, http.StatusNotFound)
	})
	if err := runCmd("anomalies", "disconnect", testGatewayUUID, testFlagDuration, "5"); err == nil {
		t.Error(errExpected404)
	}
}

func TestAnomaliesNetworkDegradation_ServerError(t *testing.T) {
	newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, bodyNotFound, http.StatusNotFound)
	})
	if err := runCmd("anomalies", cmdNetDegradation, testGatewayUUID, testFlagDuration, "5"); err == nil {
		t.Error(errExpected404)
	}
}

func TestAnomaliesOutlier_ServerError(t *testing.T) {
	newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, bodyNotFound, http.StatusNotFound)
	})
	if err := runCmd("anomalies", "outlier", "42"); err == nil {
		t.Error(errExpected404)
	}
}

// ── Shell edge cases ──────────────────────────────────────────────────────────

func TestShellQuitCommand(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf(fmtPipeErr, err)
	}
	originalStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = originalStdin })
	t.Cleanup(func() { _ = r.Close() })

	if _, err := w.WriteString("quit\n"); err != nil {
		t.Fatalf(fmtWriteErr, err)
	}
	_ = w.Close()

	if err := runCmd("shell"); err != nil {
		t.Fatalf("shell quit failed: %v", err)
	}
}

func TestShellEmptyLineAndNestedShell(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf(fmtPipeErr, err)
	}
	originalStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = originalStdin })
	t.Cleanup(func() { _ = r.Close() })

	if _, err := w.WriteString("\nshell\nexit\n"); err != nil {
		t.Fatalf(fmtWriteErr, err)
	}
	_ = w.Close()

	if err := runCmd("shell"); err != nil {
		t.Fatalf("shell with empty line and nested shell attempt failed: %v", err)
	}
}

func TestShellEOF(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf(fmtPipeErr, err)
	}
	originalStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = originalStdin })
	t.Cleanup(func() { _ = r.Close() })

	// Close writer immediately to trigger EOF.
	_ = w.Close()

	if err := runCmd("shell"); err != nil {
		t.Fatalf("shell EOF failed: %v", err)
	}
}

func TestShellCommandError(t *testing.T) {
	newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf(fmtPipeErr, err)
	}
	originalStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = originalStdin })
	t.Cleanup(func() { _ = r.Close() })

	if _, err := w.WriteString("gateways list\nexit\n"); err != nil {
		t.Fatalf(fmtWriteErr, err)
	}
	_ = w.Close()

	// Shell should not return an error even when a sub-command fails.
	if err := runCmd("shell"); err != nil {
		t.Fatalf("shell should swallow sub-command errors: %v", err)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	originalStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf(fmtPipeErr, err)
	}
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = originalStdout
	})

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read captured output: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("close reader: %v", err)
	}

	return string(b)
}

func TestExecuteHelp(t *testing.T) {
	rootCmd.SetArgs([]string{"--help"})
	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	if err := Execute(); err != nil {
		t.Fatalf("Execute should succeed for --help: %v", err)
	}
}

func TestShellExitImmediately(t *testing.T) {
	originalStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf(fmtPipeErr, err)
	}
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = originalStdin
	})
	t.Cleanup(func() {
		_ = r.Close()
	})

	if _, err := w.WriteString("exit\n"); err != nil {
		t.Fatalf("write shell input: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close shell input writer: %v", err)
	}

	if err := runCmd("shell"); err != nil {
		t.Fatalf("shell command failed: %v", err)
	}
}

func TestPrintPromptRawOutput(t *testing.T) {
	originalRawOutput := pterm.RawOutput
	pterm.RawOutput = true
	t.Cleanup(func() {
		pterm.RawOutput = originalRawOutput
	})

	out := captureStdout(t, printPrompt)
	if out != "sim-cli> " {
		t.Fatalf("prompt = %q, want %q", out, "sim-cli> ")
	}
}

func TestPrintWelcomeBannerRawModeDoesNotCrash(t *testing.T) {
	originalRawOutput := pterm.RawOutput
	pterm.RawOutput = true
	t.Cleanup(func() {
		pterm.RawOutput = originalRawOutput
	})

	// In non-TTY/raw mode the renderer may bypass direct stdout writes,
	// but the banner must still render without panicking.
	printWelcomeBanner()
}

func TestStatusStyleVariants(t *testing.T) {
	originalRawOutput := pterm.RawOutput
	pterm.RawOutput = false
	t.Cleanup(func() {
		pterm.RawOutput = originalRawOutput
	})

	if got := statusStyle("connected"); got != "connected" {
		t.Fatalf("connected style = %q, want %q", got, "connected")
	}
	if got := statusStyle("disconnected"); got != "disconnected" {
		t.Fatalf("disconnected style = %q, want %q", got, "disconnected")
	}
	if got := statusStyle("unknown"); got != "unknown" {
		t.Fatalf("unknown style = %q, want %q", got, "unknown")
	}
}

func TestPrintSensorTable_EmptySlice_NoOutput(t *testing.T) {
	out := captureStdout(t, func() {
		printSensorTable([]client.Sensor{})
	})

	if out != "" {
		t.Fatalf("expected no output for empty sensor table, got %q", out)
	}
}
