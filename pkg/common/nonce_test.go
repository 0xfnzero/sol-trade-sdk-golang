package common

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/jsonrpc"
)

func TestParseNonceInfo(t *testing.T) {
	authority := solana.PublicKeyFromBytes(bytesOf(7))
	nonceHash := solana.HashFromBytes(bytesOf(9))
	nonceAccount := solana.PublicKeyFromBytes(bytesOf(3))
	data := make([]byte, 80)
	data[0], data[4] = 1, 1
	copy(data[8:40], authority[:])
	copy(data[40:72], nonceHash[:])

	got, err := parseNonceInfo(nonceAccount, data)
	if err != nil {
		t.Fatalf("parseNonceInfo returned error: %v", err)
	}
	if got.NonceAccount != nonceAccount {
		t.Fatalf("nonce account mismatch: got %s want %s", got.NonceAccount, nonceAccount)
	}
	if got.Authority != authority {
		t.Fatalf("authority mismatch: got %s want %s", got.Authority, authority)
	}
	if got.NonceHash != nonceHash {
		t.Fatalf("nonce hash mismatch: got %s want %s", got.NonceHash, nonceHash)
	}
	if got.RecentBlockhash != nonceHash {
		t.Fatalf("recent blockhash mismatch: got %s want %s", got.RecentBlockhash, nonceHash)
	}
}

func TestParseNonceInfoRejectsShortData(t *testing.T) {
	if _, err := parseNonceInfo(solana.PublicKey{}, make([]byte, 10)); err == nil {
		t.Fatal("expected an error for short nonce account data")
	}
}

func bytesOf(value byte) []byte {
	out := make([]byte, 32)
	for i := range out {
		out[i] = value
	}
	return out
}

type nonceAccountRPC struct{ account *rpc.Account }

func (c nonceAccountRPC) CallForInto(_ context.Context, out any, method string, _ []any) error {
	if method != "getAccountInfo" {
		return errors.New("unexpected RPC method")
	}
	*out.(**rpc.GetAccountInfoResult) = &rpc.GetAccountInfoResult{Value: c.account}
	return nil
}
func (nonceAccountRPC) CallWithCallback(context.Context, string, []any, func(*http.Request, *http.Response) error) error {
	return errors.New("network forbidden")
}
func (nonceAccountRPC) CallBatch(context.Context, jsonrpc.RPCRequests) (jsonrpc.RPCResponses, error) {
	return nil, errors.New("network forbidden")
}

func TestFetchNonceInfoValidatesPublicAccount(t *testing.T) {
	for _, kind := range []string{"valid", "missing", "foreign-owner", "executable", "uninitialized", "unknown-state", "legacy-version", "unknown-version", "truncated", "oversized"} {
		t.Run(kind, func(t *testing.T) {
			data := make([]byte, 80)
			data[0], data[4] = 1, 1
			copy(data[8:40], bytesOf(7))
			copy(data[40:72], bytesOf(9))
			owner := solana.SystemProgramID
			if kind == "foreign-owner" {
				owner = solana.TokenProgramID
			}
			if kind == "uninitialized" {
				data[4] = 0
			}
			if kind == "unknown-state" {
				data[4] = 2
			}
			if kind == "legacy-version" {
				data[0] = 0
			}
			if kind == "unknown-version" {
				data[0] = 2
			}
			if kind == "truncated" {
				data = data[:72]
			}
			if kind == "oversized" {
				data = append(data, 0)
			}
			account := &rpc.Account{Owner: owner, Executable: kind == "executable", Data: rpc.DataBytesOrJSONFromBytes(data)}
			if kind == "missing" {
				account = nil
			}
			got, err := FetchNonceInfo(context.Background(), rpc.NewWithCustomRPCClient(nonceAccountRPC{account}), solana.PublicKey{})
			if kind == "valid" {
				if err != nil || got == nil || got.Authority != solana.PublicKeyFromBytes(bytesOf(7)) || got.NonceHash != solana.HashFromBytes(bytesOf(9)) || got.RecentBlockhash != got.NonceHash {
					t.Fatalf("invalid success result: %v %v", got, err)
				}
			} else if kind == "missing" {
				if !errors.Is(err, rpc.ErrNotFound) || got != nil {
					t.Fatalf("missing account: %v %v", got, err)
				}
			} else if err == nil || got != nil {
				t.Fatalf("accepted %s: %v %v", kind, got, err)
			}
		})
	}
}
