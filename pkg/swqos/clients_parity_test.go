package swqos

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang/pkg"
)

func TestSwqosEndpointParityRustV4021(t *testing.T) {
	if MinTipDefault != 0.00001 {
		t.Fatalf("expected default min tip 0.00001, got %f", MinTipDefault)
	}
	if bloxrouteEndpoints[SwqosRegionSingapore] != "https://tokyo.solana.dex.blxrbdn.com" {
		t.Fatalf("unexpected Bloxroute Singapore endpoint: %s", bloxrouteEndpoints[SwqosRegionSingapore])
	}
	if node1Endpoints[SwqosRegionSingapore] != "http://tk.node1.me" {
		t.Fatalf("unexpected Node1 Singapore endpoint: %s", node1Endpoints[SwqosRegionSingapore])
	}
	if blockRazorEndpoints[SwqosRegionSingapore] != "http://singapore.solana.blockrazor.xyz:443/sendTransaction" {
		t.Fatalf("unexpected BlockRazor Singapore endpoint: %s", blockRazorEndpoints[SwqosRegionSingapore])
	}
	if astralaneEndpoints[SwqosRegionSLC] != "http://la.gateway.astralane.io/irisb" {
		t.Fatalf("unexpected Astralane SLC endpoint: %s", astralaneEndpoints[SwqosRegionSLC])
	}
	if astralaneEndpoints[SwqosRegionSingapore] != "http://sg.gateway.astralane.io/irisb" {
		t.Fatalf("unexpected Astralane Singapore endpoint: %s", astralaneEndpoints[SwqosRegionSingapore])
	}
	if astralaneQuicHosts[SwqosRegionSingapore] != "sg.gateway.astralane.io" {
		t.Fatalf("unexpected Astralane QUIC Singapore host: %s", astralaneQuicHosts[SwqosRegionSingapore])
	}
	if stelliumEndpoints[SwqosRegionSingapore] != "http://tyo1.flashrpc.com" {
		t.Fatalf("unexpected Stellium Singapore endpoint: %s", stelliumEndpoints[SwqosRegionSingapore])
	}
	if solamiEndpoints[SwqosRegionSingapore] != "beam.solami.dev:11000" {
		t.Fatalf("unexpected Solami Singapore endpoint: %s", solamiEndpoints[SwqosRegionSingapore])
	}
	if speedlandingEndpoints[SwqosRegionSingapore] != "sgp.speedlanding.trade:17778" {
		t.Fatalf("unexpected Speedlanding Singapore endpoint: %s", speedlandingEndpoints[SwqosRegionSingapore])
	}
}

func TestDefaultExtendedSwqosConfigUsesRustMevDefault(t *testing.T) {
	config := DefaultSwqosConfigExtended(SwqosTypeBlockRazor)

	if config.MevProtection != MevProtectionNone {
		t.Fatalf("expected Rust default MEV protection none, got %v", config.MevProtection)
	}
}

func TestSignatureFromSerializedTransaction(t *testing.T) {
	tx := append([]byte{1}, make([]byte, 64)...)
	for i := 1; i < len(tx); i++ {
		tx[i] = 7
	}
	sig, err := signatureFromSerializedTransaction(tx)
	if err != nil {
		t.Fatal(err)
	}
	want := "99eUso3aSbE9tqGSTXzo3TLfKb9RkMTURrHKQ1K7Zh3BbeqPevr5E1iCbpTjqHuTFLtfxTTD5ekfVuZFzQyEQf8"
	if sig.String() != want {
		t.Fatalf("expected %s, got %s", want, sig.String())
	}
}

func TestBlockRazorHTTPAcceptsPlainTextSignature(t *testing.T) {
	tx := append([]byte{1}, make([]byte, 64)...)
	for i := 1; i < len(tx); i++ {
		tx[i] = 7
	}
	want := "99eUso3aSbE9tqGSTXzo3TLfKb9RkMTURrHKQ1K7Zh3BbeqPevr5E1iCbpTjqHuTFLtfxTTD5ekfVuZFzQyEQf8"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("expected application/json content type, got %q", got)
		}
		if got := r.Header.Get("apikey"); got != "token" {
			t.Fatalf("expected apikey header, got %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatal(err)
		}
		if payload["transaction"] != base64.StdEncoding.EncodeToString(tx) || payload["mode"] != "fast" {
			t.Fatalf("unexpected payload: %#v", payload)
		}
		_, _ = w.Write([]byte(want))
	}))
	defer server.Close()

	client := NewBlockRazorClient(server.URL, "token", false)
	got, err := client.SendTransaction(context.Background(), TradeTypeBuy, tx, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != want {
		t.Fatalf("expected %s, got %s", want, got.String())
	}
}

func TestBlockRazorHTTPErrorDoesNotFallbackToSignature(t *testing.T) {
	tx := append([]byte{1}, make([]byte, 64)...)
	for i := 1; i < len(tx); i++ {
		tx[i] = 7
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewBlockRazorClient(server.URL, "token", false)
	if _, err := client.SendTransaction(context.Background(), TradeTypeBuy, tx, false); err == nil {
		t.Fatal("expected HTTP error")
	}
}

func TestAstralaneBinaryHTTPSendsRawTransactionBytes(t *testing.T) {
	tx := append([]byte{1}, make([]byte, 64)...)
	for i := 1; i < len(tx); i++ {
		tx[i] = 7
	}
	want := "99eUso3aSbE9tqGSTXzo3TLfKb9RkMTURrHKQ1K7Zh3BbeqPevr5E1iCbpTjqHuTFLtfxTTD5ekfVuZFzQyEQf8"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Content-Type"); got != "application/octet-stream" {
			t.Fatalf("expected application/octet-stream content type, got %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != string(tx) {
			t.Fatalf("unexpected raw transaction body")
		}
		if r.URL.Query().Get("api-key") != "token" || r.URL.Query().Get("method") != "sendTransaction" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewAstralaneClient(server.URL, "token")
	got, err := client.SendTransaction(context.Background(), TradeTypeBuy, tx, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != want {
		t.Fatalf("expected %s, got %s", want, got.String())
	}
}

func TestAstralaneHTTPErrorDoesNotFallbackToSignature(t *testing.T) {
	tx := append([]byte{1}, make([]byte, 64)...)
	for i := 1; i < len(tx); i++ {
		tx[i] = 7
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewAstralaneClient(server.URL, "token")
	if _, err := client.SendTransaction(context.Background(), TradeTypeBuy, tx, false); err == nil {
		t.Fatal("expected HTTP error")
	}
}

func TestFlashBlockHTTPErrorPreservesHTTPStatus(t *testing.T) {
	tx := append([]byte{1}, make([]byte, 64)...)
	for i := 1; i < len(tx); i++ {
		tx[i] = 7
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("submit failed"))
	}))
	defer server.Close()

	client := NewFlashBlockClient(server.URL, "token")
	_, err := client.SendTransaction(context.Background(), TradeTypeBuy, tx, false)
	if err == nil {
		t.Fatal("expected HTTP error")
	}
	if !strings.Contains(err.Error(), "submit failed") {
		t.Fatalf("expected provider body in error, got %v", err)
	}
}

func TestBloxrouteSuccessWithoutSignatureIsError(t *testing.T) {
	tx := append([]byte{1}, make([]byte, 64)...)
	for i := 1; i < len(tx); i++ {
		tx[i] = 7
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{})
	}))
	defer server.Close()

	client := NewBloxrouteClient(server.URL, "token")
	_, err := client.SendTransaction(context.Background(), TradeTypeBuy, tx, false)
	if err == nil {
		t.Fatal("expected missing signature error")
	}
	if !strings.Contains(err.Error(), "missing transaction signature") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSpeedlandingUsesFixedRustSNI(t *testing.T) {
	client := NewSpeedlandingClient("nyc.speedlanding.trade:17778", "")

	if client.serverName != "speed-landing" {
		t.Fatalf("expected speed-landing SNI, got %q", client.serverName)
	}
}

func TestExtendedProviderFactoryRejectsNonRustParityTypes(t *testing.T) {
	factory := &SwqosProviderFactory{}

	if _, err := factory.CreateProvider(&SwqosConfigExtended{Type: SwqosType(999)}); err == nil {
		t.Fatalf("expected unsupported provider error")
	}
}

func TestFactoriesRejectRustBlacklistedNextBlock(t *testing.T) {
	clientFactory := &ClientFactory{}
	if _, err := clientFactory.CreateClient(soltradesdk.SwqosConfig{Type: SwqosTypeNextBlock}, "https://rpc.example"); err == nil {
		t.Fatalf("expected NextBlock sender factory to reject blacklisted provider")
	}

	providerFactory := &SwqosProviderFactory{}
	if _, err := providerFactory.CreateProvider(&SwqosConfigExtended{Type: SwqosTypeNextBlock}); err == nil {
		t.Fatalf("expected NextBlock provider factory to reject blacklisted provider")
	}
}

func TestExtendedBlockRazorDelegatesToSenderRequestShape(t *testing.T) {
	tx := append([]byte{1}, make([]byte, 64)...)
	for i := 1; i < len(tx); i++ {
		tx[i] = 7
	}
	want := "99eUso3aSbE9tqGSTXzo3TLfKb9RkMTURrHKQ1K7Zh3BbeqPevr5E1iCbpTjqHuTFLtfxTTD5ekfVuZFzQyEQf8"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/submit" {
			t.Fatalf("extended provider must not use legacy /api/v1/submit")
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("expected application/json content type, got %q", got)
		}
		if got := r.Header.Get("apikey"); got != "token" {
			t.Fatalf("expected apikey header, got %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatal(err)
		}
		if payload["transaction"] != base64.StdEncoding.EncodeToString(tx) || payload["mode"] != "fast" {
			t.Fatalf("unexpected payload: %#v", payload)
		}
		_, _ = w.Write([]byte(want))
	}))
	defer server.Close()

	provider := NewBlockRazorExtClient(&SwqosConfigExtended{
		Type:   SwqosTypeBlockRazor,
		APIKey: "token",
		URL:    server.URL,
	})
	result, err := provider.SubmitTransaction(context.Background(), tx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success || result.Signature.String() != want {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestExtendedAstralaneDelegatesToBinarySenderRequestShape(t *testing.T) {
	tx := append([]byte{1}, make([]byte, 64)...)
	for i := 1; i < len(tx); i++ {
		tx[i] = 7
	}
	want := "99eUso3aSbE9tqGSTXzo3TLfKb9RkMTURrHKQ1K7Zh3BbeqPevr5E1iCbpTjqHuTFLtfxTTD5ekfVuZFzQyEQf8"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/submit" {
			t.Fatalf("extended provider must not use legacy /api/v1/submit")
		}
		if got := r.Header.Get("Content-Type"); got != "application/octet-stream" {
			t.Fatalf("expected application/octet-stream content type, got %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != string(tx) {
			t.Fatalf("unexpected raw body")
		}
		if r.URL.Query().Get("api-key") != "token" || r.URL.Query().Get("method") != "sendTransaction" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := NewAstralaneExtClient(&SwqosConfigExtended{
		Type:   SwqosTypeAstralane,
		APIKey: "token",
		URL:    server.URL,
	})
	result, err := provider.SubmitTransaction(context.Background(), tx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success || result.Signature.String() != want {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestExtendedStelliumDelegatesToSenderRequestShape(t *testing.T) {
	tx := append([]byte{1}, make([]byte, 64)...)
	for i := 1; i < len(tx); i++ {
		tx[i] = 7
	}
	want := "99eUso3aSbE9tqGSTXzo3TLfKb9RkMTURrHKQ1K7Zh3BbeqPevr5E1iCbpTjqHuTFLtfxTTD5ekfVuZFzQyEQf8"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/submit" {
			t.Fatalf("extended provider must not use legacy /api/v1/submit")
		}
		if r.URL.Path != "/token" {
			t.Fatalf("expected sender path /token, got %s", r.URL.Path)
		}
		var payload struct {
			Method string        `json:"method"`
			Params []interface{} `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.Method != "sendTransaction" {
			t.Fatalf("unexpected method: %s", payload.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"result": want})
	}))
	defer server.Close()

	factory := &SwqosProviderFactory{}
	provider, err := factory.CreateProvider(&SwqosConfigExtended{
		Type:   SwqosTypeStellium,
		APIKey: "token",
		URL:    server.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := provider.(*senderBackedExtClient).SubmitTransaction(context.Background(), tx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success || result.Signature.String() != want {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestExtendedFactoryPreservesRegionThroughSenderFactory(t *testing.T) {
	factory := &SwqosProviderFactory{}
	provider, err := factory.CreateProvider(&SwqosConfigExtended{
		Type:   SwqosTypeBloxroute,
		Region: SwqosRegionSingapore,
		APIKey: "token",
	})
	if err != nil {
		t.Fatal(err)
	}
	wrapped, ok := provider.(*senderBackedExtClient)
	if !ok {
		t.Fatalf("expected sender-backed provider, got %T", provider)
	}
	client, ok := wrapped.sender.(*BloxrouteClient)
	if !ok {
		t.Fatalf("expected Bloxroute sender, got %T", wrapped.sender)
	}
	if client.endpoint != bloxrouteEndpoints[SwqosRegionSingapore] {
		t.Fatalf("expected Singapore endpoint, got %s", client.endpoint)
	}
}

func TestExtendedFactoryPreservesHeliusSwqosOnly(t *testing.T) {
	value := true
	factory := &SwqosProviderFactory{}
	provider, err := factory.CreateProvider(&SwqosConfigExtended{
		Type:      SwqosTypeHelius,
		APIKey:    "token",
		SwqosOnly: &value,
	})
	if err != nil {
		t.Fatal(err)
	}
	wrapped, ok := provider.(*senderBackedExtClient)
	if !ok {
		t.Fatalf("expected sender-backed provider, got %T", provider)
	}
	client, ok := wrapped.sender.(*HeliusClient)
	if !ok {
		t.Fatalf("expected Helius sender, got %T", wrapped.sender)
	}
	if !client.swqosOnly {
		t.Fatal("expected swqosOnly to be preserved")
	}
}

func TestDefaultProviderTransportChains(t *testing.T) {
	factory := &ClientFactory{}
	tests := []struct {
		name         string
		config       soltradesdk.SwqosConfig
		primaryType  interface{}
		fallbackType interface{}
	}{
		{"Temporal", soltradesdk.SwqosConfig{Type: SwqosTypeTemporal, APIKey: "token"}, (*TemporalQuicClient)(nil), (*TemporalClient)(nil)},
		{"BlockRazor", soltradesdk.SwqosConfig{Type: SwqosTypeBlockRazor, APIKey: "token"}, (*BlockRazorGrpcClient)(nil), (*BlockRazorClient)(nil)},
		{"Astralane", soltradesdk.SwqosConfig{Type: SwqosTypeAstralane, APIKey: "token"}, (*AstralaneQuicClient)(nil), (*AstralaneClient)(nil)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, err := factory.CreateClient(test.config, "https://rpc.example")
			if err != nil {
				t.Fatal(err)
			}
			chain, ok := client.(*fallbackSwqosClient)
			if !ok {
				t.Fatalf("expected fallback transport chain, got %T", client)
			}
			if fmt.Sprintf("%T", chain.primary) != fmt.Sprintf("%T", test.primaryType) {
				t.Fatalf("unexpected primary transport: %T", chain.primary)
			}
			if fmt.Sprintf("%T", chain.fallback) != fmt.Sprintf("%T", test.fallbackType) {
				t.Fatalf("unexpected fallback transport: %T", chain.fallback)
			}
		})
	}
}

func TestTemporalBatchUsesBigEndianUint16Lengths(t *testing.T) {
	first := append([]byte{1}, make([]byte, 65)...)
	second := append([]byte{1}, make([]byte, 66)...)
	encoded, err := encodeTemporalBatch([][]byte{first, second})
	if err != nil {
		t.Fatal(err)
	}
	want := append([]byte{0, byte(len(first))}, first...)
	want = append(want, 0, byte(len(second)))
	want = append(want, second...)
	if string(encoded) != string(want) {
		t.Fatalf("unexpected Temporal batch framing")
	}
}

func TestFallbackPolicyOnlyAllowsTransportAndServiceErrors(t *testing.T) {
	if !shouldFallbackTransport(&TradeError{Code: 503, Message: "down"}) {
		t.Fatal("expected service failure to permit fallback")
	}
	if shouldFallbackTransport(&TradeError{Code: 401, Message: "bad key"}) {
		t.Fatal("authentication errors must not permit fallback")
	}
	if shouldFallbackTransport(errors.New("unclassified application error")) {
		t.Fatal("unclassified errors must not permit fallback")
	}
}
