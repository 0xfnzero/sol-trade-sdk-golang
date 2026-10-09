package trading

import (
	"context"
	"encoding/json"
	soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang/pkg"
	"github.com/gagliardetto/solana-go"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type closureClient struct {
	hang  bool
	calls int
}

func (c *closureClient) SendTransaction(ctx context.Context, _ soltradesdk.TradeType, _ []byte, wait bool) (solana.Signature, error) {
	if wait {
		panic("submission must not wait/observe")
	}
	c.calls++
	if c.hang {
		<-ctx.Done()
		return solana.Signature{}, ctx.Err()
	}
	return solana.Signature{1}, nil
}
func (*closureClient) SendTransactions(context.Context, soltradesdk.TradeType, [][]byte, bool) ([]solana.Signature, error) {
	return nil, nil
}
func (*closureClient) GetTipAccount() string               { return "" }
func (*closureClient) GetSwqosType() soltradesdk.SwqosType { return soltradesdk.SwqosTypeDefault }
func (*closureClient) MinTipSol() float64                  { return 0 }
func closureExecutor(t *testing.T, url string, c *closureClient) *HighPerfTradeExecutor {
	t.Helper()
	e, err := NewHighPerfTradeExecutor(&HighPerfTradeConfig{RPCUrl: url})
	if err != nil {
		t.Fatal(err)
	}
	e.clients = map[soltradesdk.SwqosType]soltradesdk.SwqosClient{soltradesdk.SwqosTypeDefault: c}
	t.Cleanup(e.Close)
	return e
}
func TestClosureDeadlineIncludesRateAndProvider(t *testing.T) {
	for _, parallel := range []bool{true, false} {
		for _, rate := range []bool{true, false} {
			e := closureExecutor(t, "http://offline.invalid", &closureClient{hang: true})
			if rate {
				e.rateLimiter = NewRateLimiter(10000)
				e.rateLimiter.lastSubmit.Store(time.Now().UnixNano())
			}
			start := time.Now()
			r := e.Execute(context.Background(), soltradesdk.TradeTypeBuy, nil, &HighPerfExecuteOptions{TimeoutMs: 10, MaxRetries: 1, ParallelSubmit: parallel})
			if r.Success || r.Error == nil || time.Since(start) > time.Second {
				t.Fatalf("deadline not honored: %+v", r)
			}
		}
	}
}
func TestClosureAcknowledgedDeadlinePreservesReceipt(t *testing.T) {
	for _, parallel := range []bool{true, false} {
		c := &closureClient{}
		e := closureExecutor(t, "http://offline.invalid", c)
		r := e.Execute(context.Background(), soltradesdk.TradeTypeBuy, nil, &HighPerfExecuteOptions{TimeoutMs: 10, MaxRetries: 3, ParallelSubmit: parallel, WaitConfirmation: true})
		if r.Success || r.Error == nil || r.Signature != (solana.Signature{1}).String() || r.ConfirmedAt != nil || r.SubmittedAt == nil {
			t.Fatalf("lost receipt: %+v", r)
		}
	}
}
func TestClosureSubmissionOnlyHasNoReadsOrConfirmedAt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("submission only performed RPC read") }))
	defer server.Close()
	e := closureExecutor(t, server.URL, &closureClient{})
	r := e.Execute(context.Background(), soltradesdk.TradeTypeBuy, nil, &HighPerfExecuteOptions{TimeoutMs: 100, ParallelSubmit: true})
	if !r.Success || r.ConfirmedAt != nil || r.ConfirmationTimeMs != 0 {
		t.Fatalf("dishonest confirmation: %+v", r)
	}
}
func TestClosureObservedConfirmation(t *testing.T) {
	for _, failed := range []bool{false, true} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var request map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&request)
			w.Header().Set("Content-Type", "application/json")
			if request["method"] == "getTransaction" {
				_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":null}`))
				return
			}
			statusErr := interface{}(nil)
			if failed {
				statusErr = "BlockhashNotFound"
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": map[string]interface{}{"context": map[string]int{"slot": 1}, "value": []interface{}{map[string]interface{}{"slot": 1, "err": statusErr, "confirmationStatus": "confirmed"}}}})
		}))
		c := &closureClient{}
		e := closureExecutor(t, server.URL, c)
		r := e.Execute(context.Background(), soltradesdk.TradeTypeBuy, nil, &HighPerfExecuteOptions{TimeoutMs: 3000, MaxRetries: 3, WaitConfirmation: true})
		server.Close()
		if r.Success == failed || (r.ConfirmedAt != nil) == failed || r.Signature == "" || c.calls != 1 {
			t.Fatalf("observation/retry incorrect: %+v calls %d", r, c.calls)
		}
	}
}
