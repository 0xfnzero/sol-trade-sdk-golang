package common

import (
	"context"
	"fmt"

	soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang/pkg"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

const nonceAccountMinLen = 72

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

	return parseNonceInfo(nonceAccount, accountInfo.Value.Data.GetBinary())
}

func parseNonceInfo(nonceAccount solana.PublicKey, data []byte) (*soltradesdk.DurableNonceInfo, error) {
	if len(data) < nonceAccountMinLen {
		return nil, fmt.Errorf("invalid nonce account data size: %d", len(data))
	}

	nonceHash := solana.HashFromBytes(data[40:72])
	return &soltradesdk.DurableNonceInfo{
		NonceAccount:    nonceAccount,
		Authority:       solana.PublicKeyFromBytes(data[8:40]),
		NonceHash:       nonceHash,
		RecentBlockhash: nonceHash,
	}, nil
}
