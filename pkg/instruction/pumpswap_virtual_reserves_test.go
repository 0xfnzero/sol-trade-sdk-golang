package instruction

import (
	"math/big"
	"testing"
)

func signedI128LE(value *big.Int) []byte {
	encoded := new(big.Int).Set(value)
	if encoded.Sign() < 0 {
		encoded.Add(encoded, new(big.Int).Lsh(big.NewInt(1), 128))
	}
	bigEndian := encoded.FillBytes(make([]byte, 16))
	littleEndian := make([]byte, 16)
	for i := range bigEndian {
		littleEndian[len(bigEndian)-1-i] = bigEndian[i]
	}
	return littleEndian
}

func TestDecodePoolVirtualQuoteReserves(t *testing.T) {
	pool := testPumpSwapParams(nil)
	body := make([]byte, 0, PoolSize)
	body = append(body, byte(1), byte(0), byte(0))
	body = append(body, pool.Pool.Bytes()...)
	body = append(body, pool.BaseMint.Bytes()...)
	body = append(body, pool.QuoteMint.Bytes()...)
	body = append(body, testPK(30).Bytes()...)
	body = append(body, pool.PoolBaseTokenAccount.Bytes()...)
	body = append(body, pool.PoolQuoteTokenAccount.Bytes()...)
	body = append(body, make([]byte, 8)...)
	body = append(body, pool.CoinCreator.Bytes()...)
	body = append(body, byte(0), byte(0))
	body = append(body, signedI128LE(big.NewInt(-123_456))...)

	decoded := DecodePool(body)
	if decoded == nil || decoded.VirtualQuoteReserves.Cmp(big.NewInt(-123_456)) != 0 {
		t.Fatalf("decoded virtual quote reserves = %v", decoded)
	}
}

func TestDecodeLegacyPoolDefaultsVirtualQuoteReservesToZero(t *testing.T) {
	body := make([]byte, LegacyPoolSize)
	decoded := DecodePool(body)
	if decoded == nil || decoded.VirtualQuoteReserves.Sign() != 0 {
		t.Fatalf("decoded legacy pool = %v", decoded)
	}
}

func TestDecodePoolRejectsPartialCurrentLayout(t *testing.T) {
	for size := LegacyPoolSize + 1; size < PoolSize; size++ {
		if DecodePool(make([]byte, size)) != nil {
			t.Fatalf("accepted partial pool body size %d", size)
		}
	}
}

func TestDecodePoolAccountValidatesDiscriminator(t *testing.T) {
	body := make([]byte, PoolSize)
	account := append(append([]byte{}, PUMPSWAP_POOL_DISCRIMINATOR...), body...)
	if DecodePoolAccount(account) == nil {
		t.Fatal("expected valid Pool account")
	}
	account[0] ^= 0xff
	if DecodePoolAccount(account) != nil {
		t.Fatal("accepted Pool account with wrong discriminator")
	}
}
