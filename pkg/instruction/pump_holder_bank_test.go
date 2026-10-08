package instruction

import (
	"encoding/hex"
	"encoding/json"
	"github.com/gagliardetto/solana-go"
	"os"
	"reflect"
	"strconv"
	"testing"
)

func TestHolderBankWires(t *testing.T) {
	b, e := os.ReadFile("testdata/pump_holder_bank_20261009.json")
	if e != nil {
		t.Fatal(e)
	}
	var f struct {
		Cases []struct {
			Name, Instruction, Program, Data string
			Roles                            map[string]string
			Args, Accounts                   []string
		}
	}
	if e = json.Unmarshal(b, &f); e != nil {
		t.Fatal(e)
	}
	for _, c := range f.Cases {
		roles := map[string]solana.PublicKey{}
		for k, v := range c.Roles {
			roles[k] = solana.MustPublicKeyFromBase58(v)
		}
		args := []uint64{}
		for _, v := range c.Args {
			n, e := strconv.ParseUint(v, 10, 64)
			if e != nil {
				t.Fatal(e)
			}
			args = append(args, n)
		}
		ix, e := BuildPumpUpgradeInstruction(c.Instruction, roles, args, nil, nil)
		if e != nil {
			t.Fatal(e)
		}
		data, e := ix.Data()
		if e != nil {
			t.Fatal(e)
		}
		keys := []string{}
		for _, a := range ix.Accounts() {
			keys = append(keys, a.PublicKey.String())
		}
		if ix.ProgramID().String() != c.Program || hex.EncodeToString(data) != c.Data || !reflect.DeepEqual(keys, c.Accounts) {
			t.Fatal(c.Name, "differs from bank wire")
		}
	}
}
