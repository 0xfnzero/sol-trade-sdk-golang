package subscription

import (
	"github.com/gagliardetto/solana-go"
	"testing"
)

func TestBatchOverlayOrderingAndFrozenBytes(t *testing.T) {
	c := &SubscriptionAccountCache{}
	k := solana.NewWallet().PublicKey()
	a := func(v uint64, data string) CachedAccount {
		return CachedAccount{Data: []byte(data), Slot: 10, WriteVersion: v}
	}
	c.Update(k, a(1, "one"))
	frozen := c.Snapshot()
	n, err := c.UpdateMany([]AccountUpdate{{k, a(2, "two")}, {k, a(1, "stale")}, {k, a(2, "two")}, {k, a(3, "three")}})
	if err != nil || n != 2 {
		t.Fatalf("changed=%d err=%v", n, err)
	}
	ctx := CacheReadContext{Slot: 10}
	current, err := c.Snapshot().Get(k, ctx, nil)
	if err != nil || string(current.Data) != "three" {
		t.Fatalf("current=%v err=%v", current, err)
	}
	old, err := frozen.Get(k, ctx, nil)
	if err != nil || string(old.Data) != "one" {
		t.Fatalf("frozen=%v err=%v", old, err)
	}
}

func TestBatchOverlayConflictDoesNotCommit(t *testing.T) {
	c := &SubscriptionAccountCache{}
	k, other := solana.NewWallet().PublicKey(), solana.NewWallet().PublicKey()
	original := CachedAccount{Data: []byte("one"), Slot: 10, WriteVersion: 1}
	c.Update(k, original)
	n, err := c.UpdateMany([]AccountUpdate{
		{other, original},
		{k, CachedAccount{Data: []byte("two"), Slot: 10, WriteVersion: 2}},
		{k, CachedAccount{Data: []byte("conflict"), Slot: 10, WriteVersion: 2}},
	})
	if err == nil || n != 0 {
		t.Fatalf("changed=%d err=%v", n, err)
	}
	// Inspect internal storage because public reads deliberately invalidate the
	// whole conflicted fork, independently of the batch rollback guarantee.
	if len(c.accounts) != 1 || string(c.accounts[k].Data) != "one" || !c.conflicted {
		t.Fatal("partial commit or missing fork invalidation")
	}
}
