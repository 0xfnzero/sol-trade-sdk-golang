// Offline replay: go run ./examples/cpmm_creator_fee_replay. No transactions are sent.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	f "github.com/0xfnzero/sol-trade-sdk-golang/pkg/instruction"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"os"
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}
func run() error {
	b, err := os.ReadFile("examples/fixtures/cpmm_creator_fee_rust_5_0_7.json")
	if err != nil {
		return err
	}
	var cases []struct {
		Name, Pool, Payer string
		Permissionless    bool
		Accounts          []*struct {
			Owner    string
			Data     string `json:"data_base64"`
			Lamports uint64
		}
		Instruction struct {
			Accounts []string
			Data     string `json:"data_base64"`
		}
	}
	if err = json.Unmarshal(b, &cases); err != nil {
		return err
	}
	for _, c := range cases {
		poolBytes, err := base64.StdEncoding.DecodeString(c.Accounts[0].Data)
		if err != nil {
			return err
		}
		pool, err := f.DecodeCpmmCollectionPool(poolBytes)
		if err != nil {
			return err
		}
		configBytes, err := base64.StdEncoding.DecodeString(c.Accounts[1].Data)
		if err != nil {
			return err
		}
		config, err := f.DecodeCpmmAmmConfig(configBytes)
		if err != nil {
			return err
		}
		var share *rpc.Account
		if a := c.Accounts[2]; a != nil {
			data, err := base64.StdEncoding.DecodeString(a.Data)
			if err != nil {
				return err
			}
			share = &rpc.Account{Owner: solana.MustPublicKeyFromBase58(a.Owner), Data: rpc.DataBytesOrJSONFromBytes(data), Lamports: a.Lamports}
		}
		rate, err := f.ResolveCreatorFeeShareRate(config, pool.PoolCreator, pool.AmmConfig, share)
		if err != nil {
			return err
		}
		var ix solana.Instruction
		if c.Permissionless {
			ix, err = f.CollectCreatorFeePermissionless(solana.MustPublicKeyFromBase58(c.Payer), solana.MustPublicKeyFromBase58(c.Pool), pool)
		} else {
			ix, err = f.CollectCreatorFee(solana.MustPublicKeyFromBase58(c.Pool), pool)
		}
		if err != nil {
			return err
		}
		if len(ix.Accounts()) != len(c.Instruction.Accounts) {
			return fmt.Errorf("account count mismatch")
		}
		for i, m := range ix.Accounts() {
			if m.PublicKey.String() != c.Instruction.Accounts[i] {
				return fmt.Errorf("account mismatch")
			}
		}
		data, err := ix.Data()
		if err != nil {
			return err
		}
		expected, err := base64.StdEncoding.DecodeString(c.Instruction.Data)
		if err != nil {
			return err
		}
		if !bytes.Equal(data, expected) {
			return fmt.Errorf("instruction data mismatch")
		}
		a, z, err := f.EstimateCreatorFeePayout(pool, rate)
		if err != nil {
			return err
		}
		fmt.Printf("%s permissionless=%v accounts=%d rate=%d payout=(%d,%d)\n", c.Name, c.Permissionless, len(ix.Accounts()), rate, a, z)
	}
	return nil
}
