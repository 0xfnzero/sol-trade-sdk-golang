package subscription

import (
	"bytes"
	"github.com/gagliardetto/solana-go"
	"testing"
)

func TestReviewGenericLaunchLab(t *testing.T) {
	c, h, _ := fixture(t)
	s := c.Snapshot()
	pool, err := s.Get(h.Pool, ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var old solana.PublicKey
	copy(old[:], pool.Data[173:205])
	p, err := s.Get(old, ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var platform solana.PublicKey
	copy(platform[:], bytes.Repeat([]byte{42}, 32))
	copy(pool.Data[173:205], platform[:])
	pool.WriteVersion++
	c.Update(h.Pool, pool)
	c.Update(platform, p)
	s = c.Snapshot()
	if _, _, err = s.StonkFunCurve(h, ctx); err == nil {
		t.Fatal("StonkFun attribution weakened")
	}
	a, _, err := s.LaunchLabCurve(h, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if a.PlatformConfig != platform {
		t.Fatal("wrong platform")
	}
	if _, err = s.PrepareRoute([]PoolTradeHint{h}, ctx, 0, solana.NewWallet().PublicKey(), 1000000000, 100, 8); err != nil {
		t.Fatal(err)
	}
}
