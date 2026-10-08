package instruction

// Offline Hook resolver for literals and AccountKey PDA seeds. Unsupported
// configurations fail closed. Refresh the mint and validation data per transfer.
import (
	"bytes"
	"encoding/binary"
	"fmt"
	solana "github.com/gagliardetto/solana-go"
)

func ResolveHookAccounts(hook, mint, mintOwner solana.PublicKey, mintData []byte, meta, metaOwner solana.PublicKey, metaData []byte, executeAccounts []solana.PublicKey) (solana.AccountMetaSlice, error) {
	fail := func(s string) (solana.AccountMetaSlice, error) { return nil, fmt.Errorf("%s", s) }
	token22 := solana.MustPublicKeyFromBase58("TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb")
	if mintOwner != token22 || len(mintData) < 166 || mintData[165] != 1 || mintData[45] != 1 {
		return fail("Invalid Token-2022 mint")
	}
	var active *solana.PublicKey
	for o := 166; o+4 <= len(mintData); {
		kind := binary.LittleEndian.Uint16(mintData[o:])
		length := int(binary.LittleEndian.Uint16(mintData[o+2:]))
		end := o + 4 + length
		if end > len(mintData) {
			return fail("Truncated mint extension")
		}
		if kind == 14 {
			if active != nil || length != 64 {
				return fail("Invalid Hook extension")
			}
			k := solana.PublicKeyFromBytes(mintData[o+36 : end])
			active = &k
		}
		o = end
	}
	if active == nil || *active != hook || hook.IsZero() {
		return fail("Active Hook program mismatch")
	}
	expected, _, err := solana.FindProgramAddress([][]byte{[]byte("extra-account-metas"), mint[:]}, hook)
	if err != nil {
		return nil, err
	}
	if meta != expected || metaOwner != hook {
		return fail("Invalid Hook validation account")
	}
	if len(executeAccounts) != 5 || executeAccounts[1] != mint || executeAccounts[4] != meta {
		return fail("Invalid Execute account order")
	}
	if len(metaData) < 16 || !bytes.Equal(metaData[:8], []byte{105, 37, 101, 197, 75, 251, 102, 26}) {
		return fail("Invalid Execute TLV")
	}
	count := uint64(binary.LittleEndian.Uint32(metaData[12:]))
	if uint64(binary.LittleEndian.Uint32(metaData[8:])) != 4+35*count || uint64(len(metaData)) != 16+35*count {
		return fail("Invalid Execute TLV length")
	}
	keys := append([]solana.PublicKey(nil), executeAccounts...)
	result := solana.AccountMetaSlice{}
	for i := 0; i < int(count); i++ {
		item := metaData[16+35*i : 51+35*i]
		config := item[1:33]
		if item[33] != 0 || item[34] > 1 {
			return fail("Unsupported signer or invalid flags")
		}
		var k solana.PublicKey
		if item[0] == 0 {
			k = solana.PublicKeyFromBytes(config)
		} else if item[0] == 1 {
			seeds := [][]byte{}
			o := 0
			for o < 32 && config[o] != 0 {
				if config[o] != 3 || o+1 >= 32 || int(config[o+1]) >= len(keys) {
					return fail("Unsupported or invalid Hook PDA seed")
				}
				index := int(config[o+1])
				seeds = append(seeds, keys[index][:])
				o += 2
			}
			for _, b := range config[o:] {
				if b != 0 {
					return fail("Invalid Hook PDA seed padding")
				}
			}
			if len(seeds) > 15 {
				return fail("Too many Hook PDA seeds")
			}
			k, _, err = solana.FindProgramAddress(seeds, hook)
			if err != nil {
				return nil, err
			}
		} else {
			return fail("Unsupported Hook account configuration")
		}
		writable := item[34] != 0
		for _, base := range executeAccounts {
			if base == k {
				writable = false
			}
		}
		duplicate, priorWritable := false, false
		for _, a := range result {
			if a.PublicKey == k {
				duplicate = true
				priorWritable = priorWritable || a.IsWritable
			}
		}
		if duplicate {
			writable = writable && priorWritable
		}
		result = append(result, &solana.AccountMeta{PublicKey: k, IsWritable: writable})
		keys = append(keys, k)
	}
	return append(result, &solana.AccountMeta{PublicKey: hook}, &solana.AccountMeta{PublicKey: meta}), nil
}
