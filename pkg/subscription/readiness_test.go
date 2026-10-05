package subscription

import (
	"errors"
	"github.com/gagliardetto/solana-go"
	"testing"
)

func TestReadinessInvalidatesFrozenSnapshotAfterGap(t *testing.T) {
	k := solana.NewWallet().PublicKey()
	owner := solana.NewWallet().PublicKey()
	r, _ := NewSubscriptionReadiness("confirmed/fork-a", []string{k.String(), "clock"})
	c := &SubscriptionAccountCache{}
	c.Update(k, CachedAccount{owner, []byte{1}, 1, 0})
	if _, err := c.ReadySnapshot(r); err == nil {
		t.Fatal("initial cache must block")
	}
	if r.MarkValidated([]string{k.String()}, 0, "confirmed/fork-a") {
		t.Fatal("clock missing")
	}
	if !r.MarkValidated([]string{"clock"}, 0, "confirmed/fork-a") {
		t.Fatal("ready failed")
	}
	old, _ := c.ReadySnapshot(r)
	ctx := CacheReadContext{1, 0, 0}
	if _, err := old.Get(k, ctx, nil); err != nil {
		t.Fatal(err)
	}
	r.Interrupt("gap")
	if _, err := old.Get(k, ctx, nil); err == nil {
		t.Fatal("frozen continuity not invalidated")
	}
	gen, _ := r.BeginRecovery("confirmed/fork-a")
	if r.MarkValidated([]string{k.String(), "clock"}, 0, "confirmed/fork-a") {
		t.Fatal("old generation accepted")
	}
	if r.MarkValidated([]string{k.String(), "clock"}, gen, "other-fork") {
		t.Fatal("wrong fork accepted")
	}
	if !r.MarkValidated([]string{k.String(), "clock"}, gen, "confirmed/fork-a") {
		t.Fatal("recovery failed")
	}
	var unavailable *CacheNotReadyError
	if err := old.AssertUsable(); !errors.As(err, &unavailable) {
		t.Fatal("old snapshot revived")
	}
	fresh, err := c.ReadySnapshot(r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = fresh.Get(k, ctx, nil); err != nil {
		t.Fatal(err)
	}
	r.RequireAccounts([]string{"new-array"})
	if _, err = c.ReadySnapshot(r); err == nil {
		t.Fatal("missing dependency accepted")
	}
}

func TestNewArrayCannotBypassForkRecovery(t *testing.T) {
	r, _ := NewSubscriptionReadiness("confirmed/fork-a", []string{"pool"})
	if !r.MarkValidated([]string{"pool"}, 0, "confirmed/fork-a") {
		t.Fatal("initial validation failed")
	}
	old, _ := r.Guard()
	r.Interrupt("fork conflict")
	generation, err := r.RequireAccounts([]string{"new-array"})
	if err != nil || r.Status().State != "continuity-broken" || r.Status().Reason != "fork conflict" {
		t.Fatal(r.Status(), err)
	}
	if r.MarkValidated([]string{"pool", "new-array"}, generation, "confirmed/fork-a") {
		t.Fatal("dependency discovery bypassed interruption")
	}
	if _, err = r.Guard(); err == nil {
		t.Fatal("broken continuity accepted")
	}
	recovered, _ := r.BeginRecovery("confirmed/fork-b")
	if r.MarkValidated([]string{"pool", "new-array"}, generation, "confirmed/fork-a") {
		t.Fatal("old generation accepted")
	}
	if r.MarkValidated([]string{"pool"}, recovered, "confirmed/fork-b") {
		t.Fatal("missing array accepted")
	}
	if !r.MarkValidated([]string{"new-array"}, recovered, "confirmed/fork-b") {
		t.Fatal("explicit recovery failed")
	}
	if _, err = r.Guard(); err != nil {
		t.Fatal(err)
	}
	if err = old(); err == nil {
		t.Fatal("old snapshot revived")
	}
}

func TestAccountConflictInvalidatesAllBoundSnapshots(t *testing.T) {
	for _, conflict := range []string{"data", "owner", "closed"} {
		t.Run(conflict, func(t *testing.T) {
			key, owner := solana.NewWallet().PublicKey(), solana.NewWallet().PublicKey()
			c := &SubscriptionAccountCache{}
			c.Update(key, CachedAccount{Owner: owner, Data: []byte("one"), Slot: 1})
			r, _ := NewSubscriptionReadiness("confirmed/fork-a", []string{key.String()})
			r.MarkValidated([]string{key.String()}, 0, "confirmed/fork-a")
			offline := c.Snapshot()
			live, err := c.ReadySnapshot(r)
			if err != nil {
				t.Fatal(err)
			}
			bad := CachedAccount{Owner: owner, Data: []byte("one"), Slot: 1}
			switch conflict {
			case "data":
				bad.Data = []byte("other")
			case "owner":
				bad.Owner = solana.NewWallet().PublicKey()
			case "closed":
				bad.Data = nil
			}
			if _, err = c.Update(key, bad); err == nil {
				t.Fatal("conflict accepted")
			}
			ctx := CacheReadContext{Slot: 1}
			for _, s := range []*AccountCacheSnapshot{offline, live, c.Snapshot()} {
				if err = s.AssertUsable(); err == nil {
					t.Fatal("snapshot remained usable")
				}
				if _, err = s.GetOptional(key, ctx, nil); err == nil {
					t.Fatal("optional read bypassed conflict")
				}
			}
			generation, _ := r.BeginRecovery("confirmed/fork-b")
			if !r.MarkValidated([]string{key.String()}, generation, "confirmed/fork-b") {
				t.Fatal("selected fork not validated")
			}
			if _, err = c.ReadySnapshot(r); err == nil {
				t.Fatal("readiness recovery erased cache conflict")
			}
			fresh := &SubscriptionAccountCache{}
			fresh.Update(key, CachedAccount{Owner: owner, Data: []byte("selected"), Slot: 1})
			s, err := fresh.ReadySnapshot(r)
			if err != nil {
				t.Fatal(err)
			}
			a, err := s.Get(key, ctx, nil)
			if err != nil || string(a.Data) != "selected" {
				t.Fatal(a, err)
			}
		})
	}
}
