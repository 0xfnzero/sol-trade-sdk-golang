package instruction

import (
	"encoding/base64"
	"encoding/json"
	"github.com/gagliardetto/solana-go"
	"os"
	"strconv"
	"testing"
)

func TestToken2022BankMintFees(t *testing.T) {
	raw, err := os.ReadFile("testdata/token2022_bank_fee_20261008.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Name, Data, Owner, Epoch string
			BasisPoints              uint16 `json:"basis_points"`
			MaximumFee               string `json:"maximum_fee"`
			Samples                  []struct{ Amount, Fee string }
		}
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) == 0 {
		t.Fatal("empty bank fixture")
	}
	number := func(s string) uint64 {
		v, e := strconv.ParseUint(s, 10, 64)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	for _, row := range fixture.Cases {
		t.Run(row.Name, func(t *testing.T) {
			data, e := base64.StdEncoding.DecodeString(row.Data)
			if e != nil {
				t.Fatal(e)
			}
			fee, e := TokenTransferFeeForEpoch(data, solana.MustPublicKeyFromBase58(row.Owner), number(row.Epoch))
			if e != nil {
				t.Fatal(e)
			}
			if fee.BasisPoints != row.BasisPoints || fee.MaximumFee != number(row.MaximumFee) {
				t.Fatal("bank fee configuration mismatch", fee)
			}
			for _, sample := range row.Samples {
				got, e := fee.Calculate(number(sample.Amount), false)
				if e != nil || got != number(sample.Fee) {
					t.Fatal("bank fee rounding/cap mismatch", sample, got, e)
				}
			}
		})
	}
}
