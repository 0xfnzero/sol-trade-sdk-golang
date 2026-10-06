package common

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

func TestTradingUtilsUseLatestBlockhash(t *testing.T) {
	for _, closeAccount := range []bool{false, true} {
		name, firstMethod := "transfer", "getBalance"
		if closeAccount {
			name, firstMethod = "close", "getAccountInfo"
		}
		t.Run(name, func(t *testing.T) {
			blockhash := solana.HashFromBytes(bytesOf(9))
			signature := solana.SignatureFromBytes(make([]byte, 64))
			var methods []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct {
					ID     json.RawMessage   `json:"id"`
					Method string            `json:"method"`
					Params []json.RawMessage `json:"params"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					return
				}
				methods = append(methods, request.Method)
				var result any
				switch request.Method {
				case "getBalance":
					result = map[string]any{"context": map[string]any{"slot": 1}, "value": 10000000}
				case "getAccountInfo":
					result = map[string]any{"context": map[string]any{"slot": 1}, "value": map[string]any{
						"lamports": 2039280, "owner": TokenProgramID, "executable": false, "rentEpoch": 0, "data": []string{"", "base64"},
					}}
				case "getLatestBlockhash":
					result = map[string]any{"context": map[string]any{"slot": 1}, "value": map[string]any{
						"blockhash": blockhash.String(), "lastValidBlockHeight": 100,
					}}
				case "sendTransaction":
					var encoded string
					if err := json.Unmarshal(request.Params[0], &encoded); err != nil {
						t.Error(err)
						return
					}
					tx, err := solana.TransactionFromBase64(encoded)
					if err != nil {
						t.Error(err)
						return
					}
					if tx.Message.RecentBlockhash != blockhash {
						t.Error("transaction did not use the latest blockhash")
					}
					if err := tx.VerifySignatures(); err != nil {
						t.Error(err)
					}
					result = signature.String()
				default:
					t.Errorf("unexpected RPC method: %s", request.Method)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result}); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			utils := NewTradingUtils(rpc.New(server.URL))
			payer := solana.NewWallet().PrivateKey
			var got solana.Signature
			var err error
			if closeAccount {
				got, err = utils.CloseTokenAccount(context.Background(), payer, solana.PublicKeyFromBytes(bytesOf(3)))
			} else {
				got, err = utils.TransferSOL(context.Background(), payer, solana.PublicKeyFromBytes(bytesOf(3)), 1000)
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != signature {
				t.Fatal("unexpected transaction signature")
			}
			if want := []string{firstMethod, "getLatestBlockhash", "sendTransaction"}; !reflect.DeepEqual(methods, want) {
				t.Fatalf("RPC methods = %v, want %v", methods, want)
			}
		})
	}
}
