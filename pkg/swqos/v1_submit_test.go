package swqos

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/gagliardetto/solana-go"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestV1SubmitRawWire(t *testing.T) {
	b, e := os.ReadFile("testdata/v1_submit_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var v struct {
		Transaction string `json:"transaction"`
		Signature   string `json:"signature"`
	}
	if e = json.Unmarshal(b, &v); e != nil {
		t.Fatal(e)
	}
	raw, e := base64.StdEncoding.DecodeString(v.Transaction)
	if e != nil {
		t.Fatal(e)
	}
	sig, e := signatureFromSerializedTransaction(raw)
	if e != nil || sig.String() != v.Signature {
		t.Fatalf("signature %s %v", sig, e)
	}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var p map[string]interface{}
		if e := json.NewDecoder(r.Body).Decode(&p); e != nil {
			t.Error(e)
		}
		if p["transaction"] != v.Transaction {
			t.Error("wire altered")
		}
		w.WriteHeader(200)
	}))
	defer server.Close()
	got, e := NewCachedWireSubmit(NewBlockRazorClient(server.URL, "", false))(context.Background(), raw, TradeTypeBuy)
	if e != nil || got.String() != v.Signature || calls != 1 {
		t.Fatalf("submit %s %v calls %d", got, e, calls)
	}
	bad := [][]byte{raw[:len(raw)-1], append(append([]byte{}, raw...), 0)}
	for _, c := range [][2]int{{1, 2}, {41, 65}, {4, 32}, {40, 64}, {42 + 64 + 20, 0}} {
		x := append([]byte{}, raw...)
		x[c[0]] = byte(c[1])
		bad = append(bad, x)
	}
	for _, x := range bad {
		if _, e := signatureFromSerializedTransaction(x); e == nil {
			t.Error("accepted malformed V1")
		}
	}
}

func TestCachedWireSubmitRejectsBeforeSend(t *testing.T) {
	submit := NewCachedWireSubmit(nil)
	if _, err := submit(context.Background(), nil, TradeTypeBuy); err == nil {
		t.Fatal("accepted missing raw client")
	}
}

type cachedSubmitCapture struct {
	SwqosClient
	kind  TradeType
	wait  bool
	calls int
}

func (c *cachedSubmitCapture) SendTransaction(ctx context.Context, kind TradeType, wire []byte, wait bool) (solana.Signature, error) {
	c.kind, c.wait = kind, wait
	c.calls++
	sig, err := signatureFromSerializedTransaction(wire)
	wire[0] = 0 // The adapter must pass a copy, preserving caller-owned signed bytes.
	return sig, err
}
func TestCachedWireSubmitDirectionAndPolling(t *testing.T) {
	raw := make([]byte, 138)
	raw[0], raw[1], raw[41] = 129, 1, 1
	for _, kind := range []TradeType{TradeTypeBuy, TradeTypeSell} {
		c := &cachedSubmitCapture{}
		submit := NewCachedWireSubmit(c)
		if _, err := submit(context.Background(), raw, kind); err != nil {
			t.Fatal(err)
		}
		if c.kind != kind || c.wait || c.calls != 1 || raw[0] != 129 {
			t.Fatal("changed direction, requested polling or mutated wire")
		}
		if _, err := submit(context.Background(), raw, TradeType(99)); err == nil {
			t.Fatal("accepted unknown direction")
		}
		if _, err := submit(context.Background(), raw[:len(raw)-1], kind); err == nil {
			t.Fatal("accepted malformed transaction")
		}
		if c.calls != 1 {
			t.Fatal("submitted invalid request")
		}
	}
}
