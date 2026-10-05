package common

import (
	"context"
	"encoding/json"
	"github.com/gorilla/websocket"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func batch2Receive[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(3 * time.Second):
		t.Fatal("subscription timed out")
		var zero T
		return zero
	}
}
func TestBatch2SubscriptionAckNotificationReconnect(t *testing.T) {
	var generation atomic.Uint64
	requests := make(chan WSRequest, 20)
	up := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, e := up.Upgrade(w, r, nil)
		if e != nil {
			return
		}
		defer c.Close()
		gen := generation.Add(1)
		for {
			var q WSRequest
			if c.ReadJSON(&q) != nil {
				return
			}
			requests <- q
			if q.Method == "slotSubscribe" {
				c.WriteJSON(map[string]interface{}{"jsonrpc": "2.0", "id": q.ID, "result": 800 + gen})
				c.WriteJSON(map[string]interface{}{"jsonrpc": "2.0", "method": "slotNotification", "params": map[string]interface{}{"subscription": 800 + gen, "result": map[string]interface{}{"slot": gen}}})
				if gen == 1 {
					return
				}
			} else {
				c.WriteJSON(map[string]interface{}{"jsonrpc": "2.0", "id": q.ID, "result": true})
			}
		}
	}))
	defer server.Close()
	m := NewSubscriptionManager("ws"+strings.TrimPrefix(server.URL, "http"), WithReconnectDelay(time.Millisecond), WithPingInterval(time.Millisecond*10))
	defer m.Disconnect()
	if e := m.Connect(context.Background()); e != nil {
		t.Fatal(e)
	}
	updates := make(chan uint64, 10)
	h, e := m.SubscribeSlot(func(b []byte) { var v struct{ Slot uint64 }; json.Unmarshal(b, &v); updates <- v.Slot }, nil)
	if e != nil {
		t.Fatal(e)
	}
	if batch2Receive(t, updates) != 1 || batch2Receive(t, updates) != 2 {
		t.Fatal("notifications not routed after reconnect")
	}
	if !h.IsSubscribed() {
		t.Fatal("ack did not mark subscribed")
	}
	if e = m.Unsubscribe(h); e != nil {
		t.Fatal(e)
	}
	for {
		q := batch2Receive(t, requests)
		if q.Method == "slotUnsubscribe" {
			p := q.Params.([]interface{})
			if p[0].(float64) != 802 {
				t.Fatalf("unsub used local ID: %v", p)
			}
			break
		}
	}
	if e = m.Disconnect(); e != nil {
		t.Fatal(e)
	}
	if e = m.Disconnect(); e != nil {
		t.Fatal(e)
	}
	if m.Connect(context.Background()) == nil {
		t.Fatal("closed manager reconnected")
	}
}
func TestBatch2CanceledPendingSubscription(t *testing.T) {
	request := make(chan WSRequest, 4)
	release := make(chan struct{})
	up := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, e := up.Upgrade(w, r, nil)
		if e != nil {
			return
		}
		defer c.Close()
		var q WSRequest
		if c.ReadJSON(&q) != nil {
			return
		}
		request <- q
		<-release
		c.WriteJSON(map[string]interface{}{"jsonrpc": "2.0", "id": q.ID, "result": 900})
		if c.ReadJSON(&q) == nil {
			request <- q
		}
	}))
	defer server.Close()
	m := NewSubscriptionManager("ws" + strings.TrimPrefix(server.URL, "http"))
	defer m.Disconnect()
	if e := m.Connect(context.Background()); e != nil {
		t.Fatal(e)
	}
	h, e := m.SubscribeSlot(func([]byte) { t.Error("canceled callback") }, nil)
	if e != nil {
		t.Fatal(e)
	}
	batch2Receive(t, request)
	if e = m.Unsubscribe(h); e != nil {
		t.Fatal(e)
	}
	close(release)
	q := batch2Receive(t, request)
	if q.Method != "slotUnsubscribe" || q.Params.([]interface{})[0].(float64) != 900 {
		t.Fatalf("late ack leaked subscription: %+v", q)
	}
	if h.IsActive() {
		t.Fatal("late ack reactivated handle")
	}
}
func TestBatch2ConcurrentSubscriptionWrites(t *testing.T) {
	up := websocket.Upgrader{}
	ack := make(chan struct{}, 100)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, e := up.Upgrade(w, r, nil)
		if e != nil {
			return
		}
		defer c.Close()
		for {
			var q WSRequest
			if c.ReadJSON(&q) != nil {
				return
			}
			c.WriteJSON(map[string]interface{}{"jsonrpc": "2.0", "id": q.ID, "result": q.ID + 1000})
			c.WriteJSON(map[string]interface{}{"jsonrpc": "2.0", "method": "slotNotification", "params": map[string]interface{}{"subscription": q.ID + 1000, "result": map[string]interface{}{"slot": 1}}})
		}
	}))
	defer server.Close()
	m := NewSubscriptionManager("ws"+strings.TrimPrefix(server.URL, "http"), WithPingInterval(time.Millisecond))
	defer m.Disconnect()
	if e := m.Connect(context.Background()); e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, e := m.SubscribeSlot(func([]byte) { ack <- struct{}{} }, nil); e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	for i := 0; i < 32; i++ {
		batch2Receive(t, ack)
	}
}
func TestBatch2SubscriptionErrorAndSignatureCompletion(t *testing.T) {
	m := NewSubscriptionManager("ws://unused.invalid")
	for _, payload := range []json.RawMessage{json.RawMessage("null"), json.RawMessage("true")} {
		errorsSeen := 0
		h := NewSubscriptionHandle(1, "slotSubscribe", nil, nil, func(error) { errorsSeen++ })
		m.subscriptions.Store(h.ID(), h)
		m.pending.Store(uint64(7), h)
		m.handleResponse(&WSResponse{ID: 7, Result: payload})
		if h.IsActive() || h.IsSubscribed() || errorsSeen != 1 {
			t.Fatal("invalid ack accepted")
		}
	}
	h := NewSubscriptionHandle(2, "signatureSubscribe", nil, func([]byte) {}, nil)
	h.setSubscribed()
	h.serverID.Store(333)
	m.subscriptions.Store(h.ID(), h)
	m.subIDToHandleID.Store(uint64(333), h.ID())
	m.handleMessage([]byte(`{"jsonrpc":"2.0","method":"signatureNotification","params":{"subscription":333,"result":{"context":{"slot":1},"value":"receivedSignature"}}}`))
	if !h.IsActive() {
		t.Fatal("receivedSignature closed handle")
	}
	m.handleMessage([]byte(`{"jsonrpc":"2.0","method":"signatureNotification","params":{"subscription":333,"result":{"context":{"slot":1},"value":{"err":null}}}}`))
	if h.IsActive() {
		t.Fatal("terminal signature retained handle")
	}
}
