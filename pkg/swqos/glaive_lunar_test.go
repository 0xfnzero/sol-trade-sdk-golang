package swqos

import (
	"testing"

	soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang/pkg"
)

func TestCreateLunarLanderAndGlaiveClients(t *testing.T) {
	httpTransport := soltradesdk.SwqosTransportHTTP
	factory := &ClientFactory{}

	lunar, err := factory.CreateClient(soltradesdk.SwqosConfig{
		Type:      soltradesdk.SwqosTypeLunarLander,
		Region:    soltradesdk.SwqosRegionDefault,
		APIKey:    "test-key",
		Transport: &httpTransport,
	}, "http://localhost:8899")
	if err != nil {
		t.Fatal(err)
	}
	if lunar.GetSwqosType() != soltradesdk.SwqosTypeLunarLander {
		t.Fatalf("lunar type: %v", lunar.GetSwqosType())
	}
	if lunar.MinTipSol() != MinTipLunarLander {
		t.Fatalf("lunar tip: %v", lunar.MinTipSol())
	}

	glaive, err := factory.CreateClient(soltradesdk.SwqosConfig{
		Type:          soltradesdk.SwqosTypeGlaive,
		Region:        soltradesdk.SwqosRegionDefault,
		APIKey:        "550e8400-e29b-41d4-a716-446655440000",
		Transport:     &httpTransport,
		MEVProtection: false,
	}, "http://localhost:8899")
	if err != nil {
		t.Fatal(err)
	}
	if glaive.GetSwqosType() != soltradesdk.SwqosTypeGlaive {
		t.Fatalf("glaive type: %v", glaive.GetSwqosType())
	}
	if glaive.MinTipSol() != MinTipGlaive {
		t.Fatalf("glaive tip: %v", glaive.MinTipSol())
	}
}
