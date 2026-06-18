package swqos

import (
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
	if blockRazorEndpoints[SwqosRegionSingapore] != "http://tokyo.solana.blockrazor.xyz:443/v2/sendTransaction" {
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
