package hotpath

import (
	"crypto/ed25519"
	"encoding/json"
	"os"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
)

// Opt-in distribution measurement, not a fixed hardware latency assertion.
func TestMeasureCachedSigningLatency(t *testing.T) {
	output := os.Getenv("SDK_LATENCY_OUTPUT")
	if output == "" {
		t.Skip("set SDK_LATENCY_OUTPUT for an offline latency measurement")
	}
	cfg := DefaultHotPathConfig()
	cfg.CacheTTL = time.Hour
	cfg.EnablePrefetch = false
	executor := NewHotPathExecutor(nil, cfg) // No RPC client or prefetch goroutine.
	hash := solana.Hash{88}
	executor.state.currentData.Store(&PrefetchedData{Blockhash: &hash, FetchedAt: time.Now()})
	payer := solana.PrivateKey(ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize)))
	ix := system.NewTransferInstruction(10001, payer.PublicKey(), solana.PublicKey{89}).Build()
	instructions := []solana.Instruction{ix}
	signers := []*solana.PrivateKey{&payer}
	gas := &GasFeeConfig{ComputeUnitLimit: 200000, ComputeUnitPrice: 1}
	build := func() []byte {
		tx, err := executor.BuildTransaction(payer.PublicKey(), instructions, signers, gas)
		if err != nil {
			t.Fatal(err)
		}
		wire, err := tx.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		return wire
	}
	rounds := []map[string]any{}
	for round := 0; round < 3; round++ {
		for i := 0; i < 200; i++ {
			build()
		}
		samples := make([]float64, 1000)
		var wire []byte
		for i := range samples {
			start := time.Now()
			wire = build()
			samples[i] = float64(time.Since(start).Nanoseconds()) / 1000
		}
		tx, err := solana.TransactionFromBytes(wire)
		if err != nil {
			t.Fatal(err)
		}
		if err := tx.VerifySignatures(); err != nil {
			t.Fatal(err)
		}
		if len(tx.Message.Instructions) != 3 {
			t.Fatal("missing gas or transfer instruction")
		}
		sort.Float64s(samples)
		rounds = append(rounds, map[string]any{"p50": samples[499], "p95": samples[949], "p99": samples[989], "maximum": samples[999], "samples_us": samples})
	}
	result := map[string]any{"name": "Go Trade cached legacy compile + compute budget + Ed25519 sign + wire", "unit": "microseconds", "warmup_per_round": 200, "measured_per_round": 1000, "rounds": rounds, "rpc_client": nil, "measured_rpc_calls": 0, "environment": map[string]any{"go": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH, "gomaxprocs": runtime.GOMAXPROCS(0)}, "scope": "Warm SDK CPU paths, nil RPC and stopped lifecycle; independent verify/transport/bank/landing excluded; normal optimized Go build without race."}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, encoded, 0644); err != nil {
		t.Fatal(err)
	}
}
