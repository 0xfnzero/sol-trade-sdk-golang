package exampleutil

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestSimulationResponseRejectsMissingResult(t *testing.T) {
	for _, body := range []string{`{}`, `{"result":null}`, `{"result":{"value":{}}}`, `{"result":{"value":null}}`, `{"error":{"code":-1}}`, `{"result":{"value":{"err":{"InstructionError":[0,"failure"]}}}}`, `invalid`} {
		if ValidateSimulationResponse([]byte(body)) == nil {
			t.Fatal("accepted", body)
		}
	}
	if e := ValidateSimulationResponse([]byte(`{"result":{"value":{"err":null}}}`)); e != nil {
		t.Fatal(e)
	}
}
func TestExplicitSimulationPreservesWireAndFailureEvidence(t *testing.T) {
	wire := []byte{1, 2, 3, 255}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var request struct {
			Method string
			Params []json.RawMessage
		}
		if e := json.NewDecoder(r.Body).Decode(&request); e != nil {
			t.Error(e)
		}
		if request.Method != "simulateTransaction" {
			t.Error("unexpected broadcast method", request.Method)
		}
		var encoded string
		json.Unmarshal(request.Params[0], &encoded)
		if encoded != base64.StdEncoding.EncodeToString(wire) {
			t.Error("wire changed")
		}
		var options struct {
			SigVerify      bool
			MinContextSlot uint64
		}
		json.Unmarshal(request.Params[1], &options)
		if options.SigVerify || options.MinContextSlot != 123 {
			t.Error(options)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"result":{"value":{"err":{"InstructionError":[0,"failure"]},"logs":[],"innerInstructions":[]}}}`))
	}))
	defer server.Close()
	t.Setenv("RPC_URL", server.URL)
	if e := SimulateIfRequested(nil, wire, 123); e != nil || calls != 0 {
		t.Fatal(e, calls)
	}
	path := filepath.Join(t.TempDir(), "simulation.json")
	if e := SimulateIfRequested([]string{"--simulate", "--simulation-out", path}, wire, 123); e == nil {
		t.Fatal("failed execution accepted")
	}
	if calls != 1 {
		t.Fatal(calls)
	}
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var saved struct{ Wire string }
	if e = json.Unmarshal(b, &saved); e != nil || saved.Wire != base64.StdEncoding.EncodeToString(wire) {
		t.Fatal(e, saved)
	}
	if e := SimulateIfRequested([]string{"--simulation-out", path}, wire, 123); e == nil {
		t.Fatal("implicit simulation accepted")
	}
}
