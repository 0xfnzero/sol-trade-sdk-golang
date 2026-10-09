package trading

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"

	soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang/pkg"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/jsonrpc"
)

type regressionOfflineClient struct{ sendErr error }

func (c regressionOfflineClient) SendTransaction(context.Context, soltradesdk.TradeType, []byte, bool) (solana.Signature, error) {
	return solana.Signature{1}, c.sendErr
}
func (regressionOfflineClient) SendTransactions(context.Context, soltradesdk.TradeType, [][]byte, bool) ([]solana.Signature, error) {
	return nil, nil
}
func (regressionOfflineClient) GetTipAccount() string { return "offline" }
func (regressionOfflineClient) GetSwqosType() soltradesdk.SwqosType {
	return soltradesdk.SwqosTypeDefault
}
func (regressionOfflineClient) MinTipSol() float64 { return 0 }

type regressionStatusRPC struct {
	statusErr any
	calls     atomic.Int64
}

func (c *regressionStatusRPC) CallForInto(_ context.Context, out any, method string, _ []any) error {
	c.calls.Add(1)
	switch method {
	case "getSignatureStatuses":
		*out.(**rpc.GetSignatureStatusesResult) = &rpc.GetSignatureStatusesResult{Value: []*rpc.SignatureStatusesResult{{ConfirmationStatus: rpc.ConfirmationStatusConfirmed, Err: c.statusErr}}}
	case "getTransaction":
		*out.(**rpc.GetTransactionResult) = &rpc.GetTransactionResult{Meta: &rpc.TransactionMeta{Err: c.statusErr}}
	default:
		return errors.New("unexpected offline RPC method")
	}
	return nil
}
func (*regressionStatusRPC) CallWithCallback(context.Context, string, []any, func(*http.Request, *http.Response) error) error {
	return errors.New("network forbidden")
}
func (*regressionStatusRPC) CallBatch(context.Context, jsonrpc.RPCRequests) (jsonrpc.RPCResponses, error) {
	return nil, errors.New("network forbidden")
}

func TestReviewExecutorReceiptStates(t *testing.T) {
	payer := solana.NewWallet().PublicKey()
	tx, err := solana.NewTransaction([]solana.Instruction{system.NewTransferInstruction(1, payer, solana.NewWallet().PublicKey()).Build()}, solana.Hash{}, solana.TransactionPayer(payer))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name                                        string
		wait, submitted, confirmed, success, cancel bool
		sendErr                                     error
		statusErr                                   any
	}{
		{name: "submitted-only", submitted: true, success: true},
		{name: "submission-rejected", sendErr: errors.New("provider reject")},
		{name: "confirmed", wait: true, submitted: true, confirmed: true, success: true},
		{name: "failed-confirmed", wait: true, submitted: true, statusErr: map[string]any{"InstructionError": []any{0, map[string]any{"Custom": 6001}}}},
		{name: "cancel-after-submit", wait: true, submitted: true, cancel: true},
	} {
		for _, parallel := range []bool{true, false} {
			name := tc.name
			if !parallel {
				name += "/sequential"
			}
			t.Run(name, func(t *testing.T) {
				transport := &regressionStatusRPC{statusErr: tc.statusErr}
				e := NewTradeExecutor(rpc.NewWithCustomRPCClient(transport), nil, nil)
				e.AddSwqosClient(regressionOfflineClient{sendErr: tc.sendErr})
				e.confirmationRetry = 1
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				if tc.cancel {
					cancel()
				}
				result := e.Execute(ctx, soltradesdk.TradeTypeBuy, tx, ExecuteOptions{ParallelSubmit: parallel, MaxRetries: 1, WaitConfirmation: tc.wait})
				if result.Submitted != tc.submitted || result.Confirmed != tc.confirmed || result.Success != tc.success {
					t.Fatalf("states %+v", result)
				}
				if result.ConfirmedAt.IsZero() == tc.confirmed {
					t.Fatalf("confirmation timestamp %+v", result)
				}
				if tc.submitted && result.Signature.IsZero() {
					t.Fatal("acknowledged signature must survive failed confirmation")
				}
				if tc.cancel && !errors.Is(result.Error, context.Canceled) {
					t.Fatal("cancellation cause lost", result.Error)
				}
				if !tc.success && result.Error == nil {
					t.Fatal("failure must retain error")
				}
				if !tc.wait && transport.calls.Load() != 0 {
					t.Fatal("submit-only path must not read RPC")
				}
			})
		}
	}
}

func TestReviewHighPerfClientSnapshotConcurrentMutation(t *testing.T) {
	e := &HighPerfTradeExecutor{clients: map[soltradesdk.SwqosType]soltradesdk.SwqosClient{soltradesdk.SwqosTypeDefault: regressionOfflineClient{}}, rateLimiter: NewRateLimiter(0), signatureCache: NewSignatureCache(1000)}
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 2000; i++ {
			e.RemoveClient(soltradesdk.SwqosTypeJito)
			// Same mutation lock as public AddClient, with a purely offline provider.
			e.mu.Lock()
			e.clients[soltradesdk.SwqosTypeJito] = regressionOfflineClient{}
			e.mu.Unlock()
		}
	}()
	close(start)
	for i := 0; i < 500; i++ {
		result := e.Execute(context.Background(), soltradesdk.TradeTypeBuy, []byte{}, &HighPerfExecuteOptions{ParallelSubmit: true})
		if !result.Success {
			t.Fatal(result.Error)
		}
	}
	wg.Wait()
}
