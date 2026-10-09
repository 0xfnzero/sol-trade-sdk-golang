package instruction

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"
)

func TestCachedMintTlv(t *testing.T) {
	data, err := os.ReadFile("testdata/cached_mint_tlv_20261009.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Name     string `json:"name"`
			Data     string `json:"mint_data"`
			Epoch    uint64 `json:"epoch"`
			Eligible bool   `json:"eligible"`
			Bps      uint16 `json:"basis_points"`
			Max      uint64 `json:"maximum_fee"`
		}
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, c := range fixture.Cases {
		t.Run(c.Name, func(t *testing.T) {
			d, e := base64.StdEncoding.DecodeString(c.Data)
			if e != nil {
				t.Fatal(e)
			}
			f, e := TokenTransferFeeForEpoch(d, mintToken2022Program, c.Epoch)
			if !c.Eligible {
				if e == nil {
					t.Fatal("unsafe mint accepted")
				}
				return
			}
			if e != nil || f.BasisPoints != c.Bps || f.MaximumFee != c.Max {
				t.Fatalf("fee=%+v error=%v", f, e)
			}
		})
	}
}
