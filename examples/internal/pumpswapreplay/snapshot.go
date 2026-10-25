// Package pumpswapreplay loads captured public account data for native examples.
package pumpswapreplay

import (
	"encoding/base64"
	"encoding/json"
	soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang/v10/pkg"
	"github.com/0xfnzero/sol-trade-sdk-golang/v10/pkg/subscription"
	"github.com/0xfnzero/sol-trade-sdk-golang/v10/pkg/trading"
	"github.com/gagliardetto/solana-go"
	"os"
)

func LoadSnapshot(path string, payer solana.PublicKey, buy bool) (trading.PumpSwapRouteSearch, trading.CachedTradeRequest, error) {
	var search trading.PumpSwapRouteSearch
	var request trading.CachedTradeRequest
	b, err := os.ReadFile(path)
	if err != nil {
		return search, request, err
	}
	var in struct {
		Slot, Epoch    uint64
		UnixTimestamp  uint64 `json:"unix_timestamp"`
		MaximumSlotAge uint64 `json:"maximum_slot_age"`
		Search         struct {
			Pool  string `json:"pool"`
			Token string `json:"token_mint"`
			Asset string `json:"asset_mint"`
			Pools []struct {
				Pool   string
				Input  string `json:"input_mint"`
				Output string `json:"output_mint"`
			} `json:"conversion_pools"`
		} `json:"route_search"`
		Accounts []struct {
			Address, Owner string
			Data           string `json:"data_base64"`
			Slot           uint64
		}
	}
	if err = json.Unmarshal(b, &in); err != nil {
		return search, request, err
	}
	parse := solana.PublicKeyFromBase58
	pool, err := parse(in.Search.Pool)
	if err != nil {
		return search, request, err
	}
	token, err := parse(in.Search.Token)
	if err != nil {
		return search, request, err
	}
	asset, err := parse(in.Search.Asset)
	if err != nil {
		return search, request, err
	}
	hints := []subscription.PoolTradeHint{}
	for _, h := range in.Search.Pools {
		p, e := parse(h.Pool)
		if e != nil {
			return search, request, e
		}
		i, e := parse(h.Input)
		if e != nil {
			return search, request, e
		}
		o, e := parse(h.Output)
		if e != nil {
			return search, request, e
		}
		hints = append(hints, subscription.PoolTradeHint{Pool: p, InputMint: i, OutputMint: o})
	}
	search = trading.NewPumpSwapRouteSearch(pool, token, asset, hints)
	cache := &subscription.SubscriptionAccountCache{}
	for _, a := range in.Accounts {
		k, e := parse(a.Address)
		if e != nil {
			return search, request, e
		}
		owner, e := parse(a.Owner)
		if e != nil {
			return search, request, e
		}
		data, e := base64.StdEncoding.DecodeString(a.Data)
		if e != nil {
			return search, request, e
		}
		if _, e = cache.Update(k, subscription.CachedAccount{Owner: owner, Data: data, Slot: a.Slot}); e != nil {
			return search, request, e
		}
	}
	direction, amount := soltradesdk.TradeTypeBuy, uint64(1000000)
	if !buy {
		direction, amount = soltradesdk.TradeTypeSell, 1000000000
	}
	slip := uint16(100)
	request = trading.CachedTradeRequest{Snapshot: cache.Snapshot(), Payer: payer, TradeType: direction, Amount: amount, Context: subscription.CacheReadContext{Slot: in.Slot, Epoch: in.Epoch, MaximumSlotAge: in.MaximumSlotAge}, UnixTimestamp: in.UnixTimestamp, SlippageBps: &slip, MaximumArrays: 8, TemporaryWsolSeed: "pumpswap-offline-replay", RentLamports: 2039280}
	return search, request, nil
}
