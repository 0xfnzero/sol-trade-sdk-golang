package subscription

import (
	"encoding/base64"
	"encoding/json"
	"github.com/gagliardetto/solana-go"
	"os"
	"strings"
	"testing"
)

func TestClmmEmptyExtendedBitmapReadOnce(t *testing.T) {
	data, err := os.ReadFile("testdata/clmm_empty_bitmap_review_20261005.json")
	if err != nil {
		t.Fatal(err)
	}
	var v clmmFixture
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	snapshot := clmmSnapshot(t, v)
	guard := snapshot.continuityGuard
	reads := 0
	snapshot.continuityGuard = func() error { reads++; return guard() }
	_, _, _, err = snapshot.PrepareClmm(PoolTradeHint{solana.MustPublicKeyFromBase58(v.Pool), solana.MustPublicKeyFromBase58(v.InputMint), solana.MustPublicKeyFromBase58(v.OutputMint)}, CacheReadContext{clmmNumber(t, v.ReadSlot), clmmNumber(t, v.Epoch), 0}, clmmNumber(t, v.UnixTimestamp), solana.MustPublicKeyFromBase58(v.Payer), 10000, 100, 8)
	if err == nil || !strings.Contains(err.Error(), "insufficient CLMM liquidity") {
		t.Fatal(err)
	}
	// Pool, config, two mints and one bitmap; no initialized tick arrays.
	if reads != 5 {
		t.Fatalf("unexpected account reads: %d", reads)
	}
}

func TestClmmExtendedBitmapValidation(t *testing.T) {
	for _, kind := range []string{"missing", "stale", "owner", "identity"} {
		t.Run(kind, func(t *testing.T) {
			data, err := os.ReadFile("testdata/clmm_empty_bitmap_review_20261005.json")
			if err != nil {
				t.Fatal(err)
			}
			var v clmmFixture
			if err := json.Unmarshal(data, &v); err != nil {
				t.Fatal(err)
			}
			last := len(v.Accounts) - 1
			match := kind
			switch kind {
			case "missing":
				v.Accounts = v.Accounts[:last]
				match = "missing cached"
			case "stale":
				v.Accounts[last].Slot = "0"
			case "owner":
				v.Accounts[last].Owner = solana.PublicKey{}.String()
			case "identity":
				b, err := base64.StdEncoding.DecodeString(v.Accounts[last].Data)
				if err != nil {
					t.Fatal(err)
				}
				b[8] ^= 1
				v.Accounts[last].Data = base64.StdEncoding.EncodeToString(b)
				match = "bitmap extension"
			}
			_, _, _, err = clmmSnapshot(t, v).PrepareClmm(PoolTradeHint{solana.MustPublicKeyFromBase58(v.Pool), solana.MustPublicKeyFromBase58(v.InputMint), solana.MustPublicKeyFromBase58(v.OutputMint)}, CacheReadContext{clmmNumber(t, v.ReadSlot), clmmNumber(t, v.Epoch), 0}, clmmNumber(t, v.UnixTimestamp), solana.MustPublicKeyFromBase58(v.Payer), 10000, 100, 8)
			if err == nil || !strings.Contains(err.Error(), match) {
				t.Fatal(err)
			}
		})
	}
}
