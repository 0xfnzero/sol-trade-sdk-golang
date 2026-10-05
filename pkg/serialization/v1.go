package serialization

// Native SIMD-0385 compiler; no RPC or Rust dependency.
import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"errors"
	"github.com/gagliardetto/solana-go"
	"github.com/mr-tron/base58"
	"sort"
)

type V1Config struct {
	PriorityFee                 *uint64
	ComputeUnitLimit            *uint32
	LoadedAccountsDataSizeLimit *uint32
	HeapSize                    *uint32
}
type CompiledV1Message struct {
	Message            []byte
	AccountKeys        []solana.PublicKey
	RequiredSignatures int
}
type v1KeyMeta struct{ Signer, Writable bool }

func CompileV1Message(payer solana.PublicKey, instructions []solana.Instruction, blockhash solana.Hash, config V1Config) (*CompiledV1Message, error) {
	if len(instructions) > 64 {
		return nil, errors.New("V1 supports at most 64 instructions")
	}
	meta := map[solana.PublicKey]v1KeyMeta{}
	add := func(k solana.PublicKey, s, w bool) {
		prior := meta[k]
		meta[k] = v1KeyMeta{s || prior.Signer, w || prior.Writable}
	}
	for _, ix := range instructions {
		add(ix.ProgramID(), false, false)
		for _, a := range ix.Accounts() {
			add(a.PublicKey, a.IsSigner, a.IsWritable)
		}
	}
	add(payer, true, true)
	delete(meta, payer)
	ordered := []solana.PublicKey{}
	for k := range meta {
		ordered = append(ordered, k)
	}
	sort.Slice(ordered, func(i, j int) bool { return bytes.Compare(ordered[i][:], ordered[j][:]) < 0 })
	sw, sr, uw, ur := []solana.PublicKey{payer}, []solana.PublicKey{}, []solana.PublicKey{}, []solana.PublicKey{}
	for _, k := range ordered {
		m := meta[k]
		switch {
		case m.Signer && m.Writable:
			sw = append(sw, k)
		case m.Signer:
			sr = append(sr, k)
		case m.Writable:
			uw = append(uw, k)
		default:
			ur = append(ur, k)
		}
	}
	all := append(append(append(sw, sr...), uw...), ur...)
	required := len(sw) + len(sr)
	if len(all) > 64 || required > 12 {
		return nil, errors.New("V1 account/signature limit")
	}
	indices := map[solana.PublicKey]byte{}
	for i, k := range all {
		indices[k] = byte(i)
	}
	mask := uint32(0)
	values := []byte{}
	put32 := func(v uint32) { values = binary.LittleEndian.AppendUint32(values, v) }
	if config.PriorityFee != nil {
		mask |= 3
		values = binary.LittleEndian.AppendUint64(values, *config.PriorityFee)
	}
	if config.ComputeUnitLimit != nil {
		mask |= 4
		put32(*config.ComputeUnitLimit)
	}
	if config.LoadedAccountsDataSizeLimit != nil {
		mask |= 8
		put32(*config.LoadedAccountsDataSizeLimit)
	}
	if config.HeapSize != nil {
		v := *config.HeapSize
		if v < 32768 || v > 262144 || v%1024 != 0 {
			return nil, errors.New("invalid V1 heap size")
		}
		mask |= 16
		put32(v)
	}
	headers, payloads := []byte{}, []byte{}
	for _, ix := range instructions {
		program := indices[ix.ProgramID()]
		if program == 0 {
			return nil, errors.New("V1 fee payer cannot be a program")
		}
		data, err := ix.Data()
		if err != nil {
			return nil, err
		}
		accounts := ix.Accounts()
		if len(accounts) > 255 || len(data) > 65535 {
			return nil, errors.New("V1 instruction payload limit")
		}
		headers = append(headers, program, byte(len(accounts)))
		headers = binary.LittleEndian.AppendUint16(headers, uint16(len(data)))
		for _, a := range accounts {
			payloads = append(payloads, indices[a.PublicKey])
		}
		payloads = append(payloads, data...)
	}
	message := []byte{129, byte(required), byte(len(sr)), byte(len(ur))}
	message = binary.LittleEndian.AppendUint32(message, mask)
	message = append(message, blockhash[:]...)
	message = append(message, byte(len(instructions)), byte(len(all)))
	for _, k := range all {
		message = append(message, k[:]...)
	}
	message = append(message, values...)
	message = append(message, headers...)
	message = append(message, payloads...)
	if len(message)+required*64 > 4096 {
		return nil, errors.New("V1 exceeds 4096 bytes")
	}
	return &CompiledV1Message{message, all, required}, nil
}
func SignV1Transaction(compiled *CompiledV1Message, signers []solana.PrivateKey) ([]byte, error) {
	if compiled == nil || compiled.RequiredSignatures < 1 || compiled.RequiredSignatures > 12 || compiled.RequiredSignatures > len(compiled.AccountKeys) || len(compiled.AccountKeys) > 64 || len(compiled.Message) < 42+32*len(compiled.AccountKeys) || len(compiled.Message)+compiled.RequiredSignatures*64 > 4096 {
		return nil, errors.New("invalid compiled V1 message")
	}
	m, count := compiled.Message, compiled.RequiredSignatures
	if m[0] != 129 || int(m[1]) != count || int(m[41]) != len(compiled.AccountKeys) || int(m[2]) >= count || int(m[3]) > len(compiled.AccountKeys)-count {
		return nil, errors.New("invalid compiled V1 signer metadata")
	}
	for i, k := range compiled.AccountKeys {
		if !bytes.Equal(m[42+32*i:74+32*i], k[:]) {
			return nil, errors.New("invalid compiled V1 signer metadata")
		}
	}
	provided := map[solana.PublicKey]solana.PrivateKey{}
	for _, s := range signers {
		if len(s) != ed25519.PrivateKeySize {
			return nil, errors.New("invalid Ed25519 key")
		}
		provided[s.PublicKey()] = s
	}
	out := append([]byte{}, compiled.Message...)
	for _, k := range compiled.AccountKeys[:compiled.RequiredSignatures] {
		s, ok := provided[k]
		if !ok {
			return nil, errors.New("missing V1 signer: " + base58.Encode(k[:]))
		}
		sig := ed25519.Sign(ed25519.PrivateKey(s), compiled.Message)
		if !ed25519.Verify(ed25519.PublicKey(k[:]), compiled.Message, sig) {
			return nil, errors.New("V1 signer mismatch")
		}
		out = append(out, sig...)
	}
	return out, nil
}
