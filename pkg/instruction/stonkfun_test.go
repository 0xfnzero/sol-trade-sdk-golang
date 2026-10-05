package instruction

import (
	"encoding/binary"
	"github.com/gagliardetto/solana-go"
	"testing"
)

func TestStonkFunStockToken2022(t *testing.T) {
	pk := func(n byte) solana.PublicKey {
		var k solana.PublicKey
		for i := range k {
			k[i] = n
		}
		return k
	}
	token2022 := solana.MustPublicKeyFromBase58("TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb")
	p := StonkFunCurveAccounts{pk(1), pk(2), pk(3), pk(4), pk(5), pk(6), stonkFunReward, solana.TokenProgramID, token2022, pk(7), pk(8)}
	for _, buy := range []bool{true, false} {
		ix, err := BuildStonkFunCurveExactIn(p, pk(9), 100, 5, buy, 0)
		if err != nil {
			t.Fatal(err)
		}
		keys := ix.Accounts()
		data, _ := ix.Data()
		if len(keys) != 18 || keys[10].PublicKey != pk(3) || keys[12].PublicKey != token2022 || binary.LittleEndian.Uint64(data[8:]) != 100 || binary.LittleEndian.Uint64(data[16:]) != 5 {
			t.Fatal("stock quote instruction mismatch")
		}
		ata, err := stonkATA(pk(9), pk(3), token2022)
		if err != nil || keys[6].PublicKey != ata {
			t.Fatal("wrong Token2022 quote ATA")
		}
	}
	p.PlatformConfig = pk(10)
	if _, err := BuildStonkFunCurveExactIn(p, pk(9), 100, 5, true, 0); err == nil {
		t.Fatal("foreign config accepted")
	}
}
