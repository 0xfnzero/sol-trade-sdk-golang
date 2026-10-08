package instruction

import (
	"encoding/base64"
	"encoding/json"
	"github.com/gagliardetto/solana-go"
	"os"
	"strings"
	"testing"
)

func TestActualBankHookMint(t *testing.T) {
	b, err := os.ReadFile("testdata/token_hook_bank_20261008.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Cases []struct {
			Name   string
			Data   string
			Owner  string
			Reject bool `json:"reject_hook"`
		}
	}
	if err = json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Cases) != 3 {
		t.Fatal("case count")
	}
	for _, c := range f.Cases {
		t.Run(c.Name, func(t *testing.T) {
			data, err := base64.StdEncoding.DecodeString(c.Data)
			if err != nil {
				t.Fatal(err)
			}
			fee, err := TokenTransferFeeForEpoch(data, solana.MustPublicKeyFromBase58(c.Owner), 0)
			if c.Reject {
				if err == nil || !strings.Contains(strings.ToLower(err.Error()), "hook") {
					t.Fatal("active Hook accepted", err)
				}
			} else if err != nil || fee.BasisPoints != 0 || fee.MaximumFee != 0 {
				t.Fatal("inactive Hook rejected", err)
			}
		})
	}
}
