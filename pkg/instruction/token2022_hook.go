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
	return ResolveHookAccountsWithContext(hook, mint, mintOwner, mintData, meta, metaOwner, metaData, executeAccounts, nil, nil)
}

// Resolve SPL PDA seeds from this transfer's Execute data and callback account snapshots.
// Signer metadata remains unsupported.
func ResolveHookAccountsWithContext(hook, mint, mintOwner solana.PublicKey, mintData []byte, meta, metaOwner solana.PublicKey, metaData []byte, executeAccounts []solana.PublicKey, executeData []byte, accountData map[string][]byte) (solana.AccountMetaSlice, error) {
	fail := func(s string) (solana.AccountMetaSlice, error) { return nil, fmt.Errorf("%s", s) }
	if len(executeData) > 0 && (len(executeData) != 16 || !bytes.Equal(executeData[:8], []byte{105, 37, 101, 197, 75, 251, 102, 26})) {
		return fail("Invalid Execute instruction data")
	}
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
		} else if item[0] == 2 {
			switch config[0] {
			case 1:
				start := int(config[1])
				for _, b := range config[2:] {
					if b != 0 {
						return fail("Invalid instruction PubkeyData")
					}
				}
				if start+32 > len(executeData) {
					return fail("Invalid instruction PubkeyData")
				}
				k = solana.PublicKeyFromBytes(executeData[start : start+32])
			case 2:
				index, start := int(config[1]), int(config[2])
				for _, b := range config[3:] {
					if b != 0 {
						return fail("Invalid account PubkeyData")
					}
				}
				if index >= len(keys) {
					return fail("Invalid account PubkeyData")
				}
				data, ok := accountData[keys[index].String()]
				if !ok || start+32 > len(data) {
					return fail("Missing or invalid PubkeyData snapshot")
				}
				k = solana.PublicKeyFromBytes(data[start : start+32])
			default:
				return fail("Invalid PubkeyData configuration")
			}
		} else if item[0] == 1 || item[0] >= 128 {
			program := hook
			if item[0] >= 128 {
				index := int(item[0] - 128)
				if index >= len(keys) {
					return fail("Invalid external Hook PDA program index")
				}
				program = keys[index]
			}
			seeds := [][]byte{}
			o := 0
			for o < 32 && config[o] != 0 {
				switch config[o] {
				case 1:
					if o+1 >= 32 {
						return fail("Truncated literal Hook PDA seed")
					}
					length := int(config[o+1])
					if length > 32 || o+2+length > 32 {
						return fail("Invalid literal Hook PDA seed")
					}
					seeds = append(seeds, config[o+2:o+2+length])
					o += 2 + length
				case 2:
					if o+2 >= 32 {
						return fail("Truncated instruction Hook PDA seed")
					}
					start, length := int(config[o+1]), int(config[o+2])
					if len(executeData) == 0 || length > 32 || start+length > len(executeData) {
						return fail("Missing or invalid Execute seed data")
					}
					seeds = append(seeds, executeData[start:start+length])
					o += 3
				case 3:
					if o+1 >= 32 || int(config[o+1]) >= len(keys) {
						return fail("Invalid account-key Hook PDA seed")
					}
					seeds = append(seeds, keys[int(config[o+1])][:])
					o += 2
				case 4:
					if o+3 >= 32 || int(config[o+1]) >= len(keys) {
						return fail("Invalid account-data Hook PDA seed")
					}
					start, length := int(config[o+2]), int(config[o+3])
					data, ok := accountData[keys[int(config[o+1])].String()]
					if !ok || length > 32 || start+length > len(data) {
						return fail("Missing or invalid account seed data")
					}
					seeds = append(seeds, data[start:start+length])
					o += 4
				default:
					return fail("Unsupported Hook PDA seed")
				}
			}
			for _, b := range config[o:] {
				if b != 0 {
					return fail("Invalid Hook PDA seed padding")
				}
			}
			if len(seeds) > 15 {
				return fail("Too many Hook PDA seeds")
			}
			k, _, err = solana.FindProgramAddress(seeds, program)
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
