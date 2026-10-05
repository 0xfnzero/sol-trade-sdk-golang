package swqos

import (
	"context"
	"fmt"
	"github.com/gagliardetto/solana-go"
)

// NewCachedWireSubmit adapts a raw-byte client to trading.CachedWireSubmit.
// It preserves Buy/Sell and never requests RPC confirmation polling.
func NewCachedWireSubmit(client SwqosClient) func(context.Context, []byte, TradeType) (solana.Signature, error) {
	return func(ctx context.Context, wire []byte, direction TradeType) (solana.Signature, error) {
		if client == nil || (direction != TradeTypeBuy && direction != TradeTypeSell) {
			return solana.Signature{}, fmt.Errorf("raw-byte client and explicit Buy/Sell required")
		}
		expected, err := signatureFromSerializedTransaction(wire)
		if err != nil {
			return solana.Signature{}, err
		}
		if err := ctx.Err(); err != nil {
			return solana.Signature{}, err
		}
		returned, err := client.SendTransaction(ctx, direction, append([]byte{}, wire...), false)
		if err != nil {
			return solana.Signature{}, err
		}
		if returned != expected {
			return solana.Signature{}, fmt.Errorf("submission signature does not match raw transaction")
		}
		return returned, nil
	}
}
