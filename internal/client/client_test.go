package client_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/NoTIPswe/notip-simulator-cli/internal/client"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func newTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *client.Client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv, client.New(srv.URL)
}

func decodeBody(t *testing.T, r *http.Request, dst any) {
	t.Helper()
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func assertMethod(t *testing.T, r *http.Request, want string) {
	t.Helper()
	if r.Method != want {
		t.Errorf("method = %s, want %s", r.Method, want)
	}
}

func assertPath(t *testing.T, r *http.Request, want string) {
	t.Helper()
	if r.URL.Path != want {
		t.Errorf("path = %s, want %s", r.URL.Path, want)
	}
}

// ── Gateway ───────────────────────────────────────────────────────────────────

func TestCreateGateway_Success(t *testing.T) {
	want := client.Gateway{ID: 1, ManagementGatewayID: "uuid-1", Status: "online"}
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/sim/gateways")

		var req client.CreateGatewayRequest
		decodeBody(t, r, &req)
		if req.FactoryID != "fac-1" || req.SerialNumber != "SN-001" {
			t.Errorf("unexpected request body: %+v", req)
		}
		writeJSON(w, http.StatusCreated, want)
	})

	got, err := c.CreateGateway(client.CreateGatewayRequest{
		FactoryID:    "fac-1",
		FactoryKey:   "key-1",
		SerialNumber: "SN-001",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != want.ID || got.ManagementGatewayID != want.ManagementGatewayID {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestCreateGateway_ServerError(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	})
	_, err := c.CreateGateway(client.CreateGatewayRequest{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestBulkCreateGateways_AllSuccess(t *testing.T) {
	want := client.BulkCreateResponse{
		Gateways: []client.Gateway{{ID: 1}, {ID: 2}},
		Errors:   []string{"", ""},
	}
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/sim/gateways/bulk")

		var req client.BulkCreateGatewaysRequest
		decodeBody(t, r, &req)
		if req.Count != 2 {
			t.Errorf("count = %d, want 2", req.Count)
		}
		writeJSON(w, http.StatusCreated, want)
	})

	got, err := c.BulkCreateGateways(client.BulkCreateGatewaysRequest{Count: 2, FactoryID: "f", FactoryKey: "k"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Gateways) != 2 {
		t.Errorf("got %d gateways, want 2", len(got.Gateways))
	}
}

func TestBulkCreateGateways_PartialErrors_207(t *testing.T) {
	want := client.BulkCreateResponse{
		Gateways: []client.Gateway{{ID: 1}},
		Errors:   []string{"", "factory key mismatch"},
	}
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusMultiStatus, want)
	})

	got, err := c.BulkCreateGateways(client.BulkCreateGatewaysRequest{Count: 2, FactoryID: "f", FactoryKey: "k"})
	if err != nil {
		t.Fatalf("unexpected error on 207: %v", err)
	}
	if got.Errors[1] == "" {
		t.Error("expected partial error to be non-empty")
	}
}

func TestListGateways_Success(t *testing.T) {
	want := []client.Gateway{{ID: 1, Status: "online"}, {ID: 2, Status: "offline"}}
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodGet)
		assertPath(t, r, "/sim/gateways")
		writeJSON(w, http.StatusOK, want)
	})

	got, err := c.ListGateways()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d gateways, want 2", len(got))
	}
	if got[0].Status != "online" || got[1].Status != "offline" {
		t.Errorf("unexpected statuses: %v", got)
	}
}

func TestListGateways_Empty(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, []client.Gateway{})
	})
	got, err := c.ListGateways()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %d items", len(got))
	}
}

func TestGetGateway_Success(t *testing.T) {
	want := client.Gateway{ID: 42, ManagementGatewayID: "uuid-42", Status: "online"}
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodGet)
		assertPath(t, r, "/sim/gateways/uuid-42")
		writeJSON(w, http.StatusOK, want)
	})

	got, err := c.GetGateway("uuid-42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != 42 {
		t.Errorf("got ID %d, want 42", got.ID)
	}
}

func TestGetGateway_NotFound(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	_, err := c.GetGateway("unknown-uuid")
	if err == nil {
		t.Fatal("expected error on 404, got nil")
	}
}

func TestStartGateway_Success(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/sim/gateways/uuid-1/start")
		w.WriteHeader(http.StatusNoContent)
	})
	if err := c.StartGateway("uuid-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStartGateway_AlreadyRunning_409(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "already running", http.StatusConflict)
	})
	if err := c.StartGateway("uuid-1"); err == nil {
		t.Fatal("expected error on 409, got nil")
	}
}

func TestStopGateway_Success(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/sim/gateways/uuid-1/stop")
		w.WriteHeader(http.StatusNoContent)
	})
	if err := c.StopGateway("uuid-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteGateway_Success(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodDelete)
		assertPath(t, r, "/sim/gateways/uuid-1")
		w.WriteHeader(http.StatusNoContent)
	})
	if err := c.DeleteGateway("uuid-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteGateway_NotFound(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	if err := c.DeleteGateway("ghost"); err == nil {
		t.Fatal("expected error on 404, got nil")
	}
}

// ── Sensor ────────────────────────────────────────────────────────────────────

func TestAddSensor_Success(t *testing.T) {
	want := client.Sensor{ID: 10, GatewayID: 5, Type: "temperature", MinRange: 0, MaxRange: 100, Algorithm: "sine_wave"}
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/sim/gateways/5/sensors")

		var req client.AddSensorRequest
		decodeBody(t, r, &req)
		if req.Type != "temperature" || req.Algorithm != "sine_wave" {
			t.Errorf("unexpected request body: %+v", req)
		}
		writeJSON(w, http.StatusCreated, want)
	})

	got, err := c.AddSensor(5, client.AddSensorRequest{
		Type:      "temperature",
		MinRange:  0,
		MaxRange:  100,
		Algorithm: "sine_wave",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != 10 || got.Type != "temperature" {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestAddSensor_GatewayNotFound(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	_, err := c.AddSensor(999, client.AddSensorRequest{Type: "temperature", Algorithm: "constant"})
	if err == nil {
		t.Fatal("expected error on 404, got nil")
	}
}

func TestListSensors_Success(t *testing.T) {
	want := []client.Sensor{
		{ID: 1, Type: "temperature"},
		{ID: 2, Type: "humidity"},
	}
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodGet)
		assertPath(t, r, "/sim/gateways/7/sensors")
		writeJSON(w, http.StatusOK, want)
	})

	got, err := c.ListSensors(7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d sensors, want 2", len(got))
	}
}

func TestDeleteSensor_Success(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodDelete)
		assertPath(t, r, "/sim/sensors/99")
		w.WriteHeader(http.StatusNoContent)
	})
	if err := c.DeleteSensor(99); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteSensor_NotFound(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	if err := c.DeleteSensor(0); err == nil {
		t.Fatal("expected error on 404, got nil")
	}
}

// ── Anomaly ───────────────────────────────────────────────────────────────────

func TestDisconnect_Success(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/sim/gateways/uuid-1/anomaly/disconnect")

		var req client.DisconnectRequest
		decodeBody(t, r, &req)
		if req.DurationSeconds != 5 {
			t.Errorf("duration = %d, want 5", req.DurationSeconds)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	if err := c.Disconnect("uuid-1", 5); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDisconnect_GatewayNotFound(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	if err := c.Disconnect("ghost", 3); err == nil {
		t.Fatal("expected error on 404, got nil")
	}
}

func TestInjectNetworkDegradation_WithPacketLoss(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/sim/gateways/uuid-1/anomaly/network-degradation")

		var req client.NetworkDegradationRequest
		decodeBody(t, r, &req)
		if req.DurationSeconds != 10 {
			t.Errorf("duration = %d, want 10", req.DurationSeconds)
		}
		if req.PacketLossPct != 0.5 {
			t.Errorf("packet_loss_pct = %f, want 0.5", req.PacketLossPct)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	if err := c.InjectNetworkDegradation("uuid-1", 10, 0.5); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInjectNetworkDegradation_DefaultPacketLoss(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		_ = json.Unmarshal(body, &req)
		// packet_loss_pct should be omitted (zero value + omitempty)
		if _, ok := req["packet_loss_pct"]; ok {
			t.Error("packet_loss_pct should be omitted when 0 so backend applies its default")
		}
		w.WriteHeader(http.StatusNoContent)
	})
	if err := c.InjectNetworkDegradation("uuid-1", 5, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInjectOutlier_WithValue(t *testing.T) {
	val := 999.9
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/sim/sensors/42/anomaly/outlier")

		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		_ = json.Unmarshal(body, &req)
		if req["value"] != 999.9 {
			t.Errorf("value = %v, want 999.9", req["value"])
		}
		w.WriteHeader(http.StatusNoContent)
	})
	if err := c.InjectOutlier(42, &val); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInjectOutlier_NoValue(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		_ = json.Unmarshal(body, &req)
		if _, ok := req["value"]; ok {
			t.Error("value should be omitted when nil")
		}
		w.WriteHeader(http.StatusNoContent)
	})
	if err := c.InjectOutlier(42, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInjectOutlier_SensorNotFound(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	if err := c.InjectOutlier(0, nil); err == nil {
		t.Fatal("expected error on 404, got nil")
	}
}

func TestGetGateway_ErrorIncludesStatusAndBody(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("gateway id format is invalid"))
	})

	_, err := c.GetGateway("bad-id")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "backend returned 400") {
		t.Fatalf("error should include status code, got: %v", err)
	}
	if !strings.Contains(err.Error(), "gateway id format is invalid") {
		t.Fatalf("error should include backend body, got: %v", err)
	}
}

func TestCreateGateway_InvalidJSONResponse(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("{invalid-json"))
	})

	_, err := c.CreateGateway(client.CreateGatewayRequest{
		FactoryID:    "f-1",
		FactoryKey:   "k-1",
		SerialNumber: "SN-1",
	})
	if err == nil {
		t.Fatal("expected decode error, got nil")
	}
}

func TestListGateways_InvalidJSONResponse(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not-json"))
	})

	_, err := c.ListGateways()
	if err == nil {
		t.Fatal("expected decode error, got nil")
	}
}
