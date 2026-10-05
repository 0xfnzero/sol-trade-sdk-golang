package swqos

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/gagliardetto/solana-go"
	"os"
	"testing"
)

type reviewWrongClient struct {
	SwqosClient
	Calls int
}

func (c *reviewWrongClient) SendTransaction(context.Context, TradeType, []byte, bool) (solana.Signature, error) {
	c.Calls++
	return solana.Signature{}, nil
}
func TestReviewSubmitValidation(t *testing.T) {
	raw, err := os.ReadFile("testdata/review10_bad_submit.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name, Wire string
		Valid      bool
	}
	json.Unmarshal(raw, &cases)
	for _, c := range cases {
		wire, _ := base64.StdEncoding.DecodeString(c.Wire)
		_, err := signatureFromSerializedTransaction(wire)
		if (err == nil) != c.Valid {
			t.Fatalf("%s %v", c.Name, err)
		}
	}
	wire, _ := base64.StdEncoding.DecodeString(cases[0].Wire)
	client := &reviewWrongClient{}
	submit := NewCachedWireSubmit(client)
	if _, err := submit(context.Background(), wire, TradeTypeBuy); err == nil {
		t.Fatal("wrong signature accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := submit(ctx, wire, TradeTypeBuy); err != context.Canceled || client.Calls != 1 {
		t.Fatal("canceled request sent")
	}
}
