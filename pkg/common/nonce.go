package common

import (
	"context"
	"encoding/binary"
	"fmt"

	soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang/pkg"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

const nonceAccountLen = 80

// FetchNonceInfo fetches durable nonce authority and current blockhash from RPC.
// The layout matches Solana's initialized nonce account:
// version (4) + state (4) + authority (32) + blockhash (32).
func FetchNonceInfo(
	ctx context.Context,
	client *rpc.Client,
	nonceAccount solana.PublicKey,
) (*soltradesdk.DurableNonceInfo, error) {
	accountInfo, err := client.GetAccountInfo(ctx, nonceAccount)
	if err != nil {
		return nil, fmt.Errorf("failed to get nonce account info: %w", err)
	}
	if accountInfo == nil || accountInfo.Value == nil {
		return nil, nil
	}
	if accountInfo.Value.Owner != solana.SystemProgramID || accountInfo.Value.Executable {
		return nil, fmt.Errorf("nonce account must be non-executable and owned by the System Program")
	}

	return parseNonceInfo(nonceAccount, accountInfo.Value.Data.GetBinary())
}

func parseNonceInfo(nonceAccount solana.PublicKey, data []byte) (*soltradesdk.DurableNonceInfo, error) {
	if len(data) != nonceAccountLen {
		return nil, fmt.Errorf("invalid nonce account data size: %d", len(data))
	}
	// Only Current/Initialized can validate durable transactions; Legacy cannot.
	if binary.LittleEndian.Uint32(data[:4]) != 1 || binary.LittleEndian.Uint32(data[4:8]) != 1 {
		return nil, fmt.Errorf("nonce account must contain current initialized state")
	}

	nonceHash := solana.HashFromBytes(data[40:72])
	return &soltradesdk.DurableNonceInfo{
		NonceAccount:    nonceAccount,
		Authority:       solana.PublicKeyFromBytes(data[8:40]),
		NonceHash:       nonceHash,
		RecentBlockhash: nonceHash,
	}, nil
}
