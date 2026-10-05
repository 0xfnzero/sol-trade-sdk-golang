package subscription

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"github.com/gagliardetto/solana-go"
	"os"
	"strconv"
	"testing"
)

func TestDammV2LiveStateAndRejections(t *testing.T) {
	d, e := os.ReadFile("../../examples/fixtures/damm_v2_buy_mainnet_20261004.json")
	if e != nil {
		t.Fatal(e)
	}
	var v struct {
		Slot  string `json:"read_slot"`
		Epoch string
		Age   string `json:"maximum_slot_age"`
		Time  string `json:"unix_timestamp"`
		Legs  []struct {
			Pool   string
			Input  string `json:"input_mint"`
			Output string `json:"output_mint"`
		}
		Accounts []struct {
			Pubkey, Owner, Data, Slot string
			Version                   string `json:"write_version"`
		}
	}
	if e = json.Unmarshal(d, &v); e != nil {
		t.Fatal(e)
	}
	num := func(s string) uint64 {
		n, e := strconv.ParseUint(s, 10, 64)
		if e != nil {
			t.Fatal(e)
		}
		return n
	}
	h := PoolTradeHint{Pool: solana.MustPublicKeyFromBase58(v.Legs[0].Pool), InputMint: solana.MustPublicKeyFromBase58(v.Legs[0].Input), OutputMint: solana.MustPublicKeyFromBase58(v.Legs[0].Output)}
	ctx := CacheReadContext{Slot: num(v.Slot), Epoch: num(v.Epoch), MaximumSlotAge: num(v.Age)}
	for _, failure := range []string{"valid", "discriminator", "owner", "status", "activation", "flag", "vault", "stale", "missing", "continuity"} {
		t.Run(failure, func(t *testing.T) {
			m := map[solana.PublicKey]CachedAccount{}
			for _, a := range v.Accounts {
				b, e := base64.StdEncoding.DecodeString(a.Data)
				if e != nil {
					t.Fatal(e)
				}
				m[solana.MustPublicKeyFromBase58(a.Pubkey)] = CachedAccount{Owner: solana.MustPublicKeyFromBase58(a.Owner), Data: b, Slot: num(a.Slot), WriteVersion: num(a.Version)}
			}
			p := m[h.Pool]
			switch failure {
			case "discriminator":
				p.Data[0] ^= 1
			case "owner":
				p.Owner = solana.PublicKey{}
			case "status":
				p.Data[481] = 1
			case "activation":
				binary.LittleEndian.PutUint64(p.Data[472:480], ^uint64(0))
			case "flag":
				p.Data[482] = 3
			case "vault":
				k := solana.MustPublicKeyFromBase58(v.Accounts[3].Pubkey)
				a := m[k]
				a.Data[108] = 2
				m[k] = a
			case "stale":
				p.Slot = 0
			case "missing":
				delete(m, solana.MustPublicKeyFromBase58(v.Accounts[1].Pubkey))
			}
			m[h.Pool] = p
			s := &AccountCacheSnapshot{accounts: m}
			if failure == "continuity" {
				s.continuityGuard = func() error { return errors.New("continuity interrupted") }
			}
			state, e := s.DammV2(h, ctx, num(v.Time))
			if failure == "valid" {
				if e != nil || state.Pool.TokenAMint != h.OutputMint && state.Pool.TokenBMint != h.OutputMint {
					t.Fatal(state, e)
				}
				payer := solana.PublicKey{42}
				minimum := uint64(1)
				for _, buy := range []bool{true, false} {
					reads := 0
					s.continuityGuard = func() error { reads++; return nil }
					hint := h
					if !buy {
						hint.InputMint, hint.OutputMint = hint.OutputMint, hint.InputMint
					}
					route, err := s.PrepareExplicitDammV2Route([]PoolTradeHint{hint}, ctx, num(v.Time), payer, 10000, &minimum, buy)
					if err != nil {
						t.Fatal(err)
					}
					if len(route.Legs) != 1 || route.Legs[0].EstimatedNetAmountOut != nil || route.MinimumNetAmountOut != minimum {
						t.Fatal("explicit minimum presented as a quote")
					}
					if reads != 7 {
						t.Fatal("redundant cached reads", reads)
					}
				}
				reads := 0
				s.continuityGuard = func() error {
					reads++
					if reads == 7 {
						return errors.New("late continuity interruption")
					}
					return nil
				}
				if _, err := s.PrepareExplicitDammV2Route([]PoolTradeHint{h}, ctx, num(v.Time), payer, 10000, &minimum, true); err == nil {
					t.Fatal("late interruption accepted")
				}
				s.continuityGuard = nil
				if _, err := s.PrepareExplicitDammV2Route([]PoolTradeHint{h}, ctx, num(v.Time), payer, 10000, nil, true); err == nil {
					t.Fatal("missing threshold accepted")
				}
				if _, err := s.PrepareExplicitDammV2Route([]PoolTradeHint{h, h}, ctx, num(v.Time), payer, 10000, &minimum, true); err == nil {
					t.Fatal("unquoted multihop accepted")
				}
			} else if e == nil {
				t.Fatal("accepted", failure)
			}
		})
	}
}
