package subscription

import "testing"

func TestReusableOwnedGraph(t *testing.T) {
	a, b, p := candidateTestKey(1), candidateTestKey(2), candidateTestKey(10)
	hints := []PoolTradeHint{{p, a, b}}
	graph, err := NewCandidateRouteGraph(hints)
	if err != nil {
		t.Fatal(err)
	}
	hints[0].OutputMint = candidateTestKey(3)
	for i := 0; i < 2; i++ {
		err = graph.Visit(a, b, 5, func(path []PoolTradeHint) (bool, error) {
			if len(path) != 1 || path[0].OutputMint != b {
				t.Fatal(path)
			}
			path[0].OutputMint = candidateTestKey(4)
			return true, nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
func TestObservedSnapshotPreservesVersion(t *testing.T) {
	k, owner := candidateTestKey(1), candidateTestKey(2)
	c := &SubscriptionAccountCache{}
	c.Update(k, CachedAccount{owner, []byte{1}, 1, 7})
	r, _ := NewSubscriptionReadiness("fork", []string{k.String()})
	if _, err := c.ObservedSnapshot(r, 100, nil); err == nil {
		t.Fatal("unready")
	}
	r.MarkValidated([]string{k.String()}, 0, "fork")
	s, err := c.ObservedSnapshot(r, 100, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := CacheReadContext{100, 0, 0}
	account, err := s.Get(k, ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if account.Slot != 1 || account.WriteVersion != 7 {
		t.Fatal(account)
	}
	if _, err = c.Snapshot().Get(k, ctx, nil); err == nil {
		t.Fatal("ordinary snapshot refreshed")
	}
	if _, err = s.Get(k, CacheReadContext{99, 0, 0}, nil); err == nil {
		t.Fatal("future observation")
	}
	r.Interrupt("gap")
	if _, err = s.Get(k, ctx, nil); err == nil {
		t.Fatal("gap ignored")
	}
}
