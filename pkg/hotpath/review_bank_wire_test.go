package hotpath

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"os"
	"testing"
	"time"

	sdk "github.com/0xfnzero/sol-trade-sdk-golang/pkg"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
)

type parsedWireClient struct {
	t                *testing.T
	payer, recipient solana.PublicKey
	amount           uint64
	calls            int
}

func (c *parsedWireClient) SendTransaction(_ context.Context, trade sdk.TradeType, wire []byte, wait bool) (solana.Signature, error) {
	c.calls++
	if trade != sdk.TradeTypeBuy || wait {
		c.t.Fatal("submission unexpectedly requested confirmation")
	}
	tx, err := solana.TransactionFromBytes(wire)
	if err != nil {
		c.t.Fatal(err)
	}
	if err = tx.VerifySignatures(); err != nil {
		c.t.Fatal(err)
	}
	ix := tx.Message.Instructions[0]
	if tx.Message.AccountKeys[ix.ProgramIDIndex] != solana.SystemProgramID || binary.LittleEndian.Uint32(ix.Data[:4]) != 2 || binary.LittleEndian.Uint64(ix.Data[4:]) != c.amount {
		c.t.Fatal("wire transfer differs from builder")
	}
	if tx.Message.AccountKeys[ix.Accounts[0]] != c.payer || tx.Message.AccountKeys[ix.Accounts[1]] != c.recipient {
		c.t.Fatal("wire account metas differ from builder")
	}
	return tx.Signatures[0], nil
}
func (*parsedWireClient) SendTransactions(context.Context, sdk.TradeType, [][]byte, bool) ([]solana.Signature, error) {
	return nil, nil
}
func (*parsedWireClient) GetTipAccount() string       { return "" }
func (*parsedWireClient) GetSwqosType() sdk.SwqosType { return sdk.SwqosTypeJito }
func (*parsedWireClient) MinTipSol() float64          { return 0 }

func TestReviewActualSignedTransferThroughCachedHotpath(t *testing.T) {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i)
	}
	payer := solana.PrivateKey(ed25519.NewKeyFromSeed(seed))
	for i := range seed {
		seed[i] = byte(i + 32)
	}
	recipient := solana.PrivateKey(ed25519.NewKeyFromSeed(seed)).PublicKey()
	hash := solana.Hash{}
	if value := os.Getenv("SDK_BANK_BLOCKHASH"); value != "" {
		var err error
		hash, err = solana.HashFromBase58(value)
		if err != nil {
			t.Fatal(err)
		}
	}
	config := DefaultHotPathConfig()
	config.EnablePrefetch = false
	for _, parallel := range []bool{false, true} {
		t.Run(map[bool]string{false: "sequential", true: "parallel"}[parallel], func(t *testing.T) {
			executor := NewHotPathExecutor(nil, config) // No RPC object exists during build/sign/submit.
			executor.state.currentData.Store(&PrefetchedData{Blockhash: &hash, FetchedAt: time.Now()})
			tx, err := executor.BuildTransaction(payer.PublicKey(), []solana.Instruction{system.NewTransferInstruction(100001, payer.PublicKey(), recipient).Build()}, []*solana.PrivateKey{&payer}, nil)
			if err != nil {
				t.Fatal(err)
			}
			client := &parsedWireClient{t: t, payer: payer.PublicKey(), recipient: recipient, amount: 100001}
			executor.AddSwqosClient(client)
			if parallel {
				executor.AddSwqosClient(&parsedWireClient{t: t, payer: payer.PublicKey(), recipient: recipient, amount: 100001})
			}
			result := executor.Execute(context.Background(), sdk.TradeTypeBuy, tx, ExecuteOptions{ParallelSubmit: parallel, Timeout: time.Second})
			if !result.Success || result.Signature != tx.Signatures[0] {
				t.Fatal(result)
			}
			if path := os.Getenv("SDK_BANK_EXPORT"); path != "" && !parallel {
				wire, err := tx.MarshalBinary()
				if err != nil {
					t.Fatal(err)
				}
				data, err := json.Marshal(map[string]any{"sdk": "trade-go", "wire": base64.StdEncoding.EncodeToString(wire), "payer": payer.PublicKey().String(), "recipient": recipient.String(), "lamports": 100001, "blockhash": hash.String()})
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
