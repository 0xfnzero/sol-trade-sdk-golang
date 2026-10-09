package hotpath

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/jsonrpc"
)

type callbackLifecycleRPC struct{ calls atomic.Uint64 }

func (c *callbackLifecycleRPC) CallForInto(_ context.Context, out any, method string, _ []any) error {
	if method != "getLatestBlockhash" {
		return errors.New("unexpected RPC method")
	}
	height := c.calls.Add(1)
	*out.(**rpc.GetLatestBlockhashResult) = &rpc.GetLatestBlockhashResult{Value: &rpc.LatestBlockhashResult{Blockhash: solana.Hash{byte(height)}, LastValidBlockHeight: height}}
	return nil
}
func (*callbackLifecycleRPC) CallWithCallback(context.Context, string, []any, func(*http.Request, *http.Response) error) error {
	return errors.New("network forbidden")
}
func (*callbackLifecycleRPC) CallBatch(context.Context, jsonrpc.RPCRequests) (jsonrpc.RPCResponses, error) {
	return nil, errors.New("network forbidden")
}

func callbackTestState() (*HotPathState, *callbackLifecycleRPC) {
	client := &callbackLifecycleRPC{}
	config := DefaultHotPathConfig()
	config.BlockhashRefreshInterval = time.Hour
	return NewHotPathState(rpc.NewWithCustomRPCClient(client), config), client
}

func awaitCallback(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("callback/lifecycle did not complete")
	}
}

func TestCallbackCanStopAndRestartState(t *testing.T) {
	state, _ := callbackTestState()
	defer state.Stop()
	done := make(chan struct{})
	var callbacks atomic.Int32
	state.OnBlockhashUpdate(func(*solana.Hash, uint64) {
		state.Stop()
		if callbacks.Add(1) == 1 {
			if err := state.Start(context.Background()); err != nil {
				t.Error(err)
			}
		} else {
			close(done)
		}
	})
	started := make(chan error, 1)
	go func() { started <- state.Start(context.Background()) }()
	awaitCallback(t, done)
	select {
	case err := <-started:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("initial Start blocked on callback")
	}
	if callbacks.Load() != 2 {
		t.Fatalf("callbacks=%d", callbacks.Load())
	}
}

func TestCallbackCoalescesWithoutBlockingPrefetchOrMutatingCache(t *testing.T) {
	state, _ := callbackTestState()
	defer state.Stop()
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var callbacks atomic.Int32
	state.OnBlockhashUpdate(func(hash *solana.Hash, height uint64) {
		if callbacks.Add(1) == 1 {
			hash[0] = 255
			close(entered)
			<-release
		} else {
			if height != 101 {
				t.Errorf("pending callback height=%d, want latest 101", height)
			}
			close(done)
		}
	})
	if err := state.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	awaitCallback(t, entered)
	hash, _, valid := state.GetBlockhash()
	if !valid || hash[0] != 1 {
		t.Fatal("callback mutated the cached hash")
	}
	state.lifecycleMu.Lock()
	run := state.prefetchRun
	state.lifecycleMu.Unlock()
	for i := 0; i < 100; i++ {
		if err := state.prefetchBlockhash(run.ctx); err != nil {
			t.Fatal(err)
		}
	}
	state.mu.Lock()
	running, pending := state.callbackRunning, state.pendingCallback
	state.mu.Unlock()
	if !running || pending == nil || pending.lastValidHeight != 101 || callbacks.Load() != 1 {
		t.Fatal("notification backlog was not coalesced")
	}
	close(release)
	awaitCallback(t, done)
	if callbacks.Load() != 2 {
		t.Fatal("more than one pending callback was retained")
	}
}

func TestStopDropsPendingCallbackAndDoesNotWaitForActiveCallback(t *testing.T) {
	state, _ := callbackTestState()
	entered, release := make(chan struct{}), make(chan struct{})
	var callbacks atomic.Int32
	state.OnBlockhashUpdate(func(*solana.Hash, uint64) {
		if callbacks.Add(1) == 1 {
			close(entered)
			<-release
		}
	})
	if err := state.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	awaitCallback(t, entered)
	state.lifecycleMu.Lock()
	run := state.prefetchRun
	state.lifecycleMu.Unlock()
	if err := state.prefetchBlockhash(run.ctx); err != nil {
		t.Fatal(err)
	}
	stopped := make(chan struct{})
	go func() { state.Stop(); close(stopped) }()
	awaitCallback(t, stopped)
	close(release)
	// Wait for the one owned notification worker to drain the cancelled update.
	expires := time.Now().Add(time.Second)
	for {
		state.mu.Lock()
		running := state.callbackRunning
		state.mu.Unlock()
		if !running {
			break
		}
		if time.Now().After(expires) {
			t.Fatal("notification worker did not stop")
		}
		time.Sleep(time.Millisecond)
	}
	if callbacks.Load() != 1 {
		t.Fatal("cancelled pending update was delivered")
	}
}
