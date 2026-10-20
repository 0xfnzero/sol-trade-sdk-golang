package stockcatalog

import (
	_ "embed"
	"encoding/json"
)

//go:embed stock_token_catalog.json
var bundledStockCatalog []byte

type StockDexType string

const (
	StockRaydiumClmm   StockDexType = "raydium_clmm"
	StockRaydiumCpmm   StockDexType = "raydium_cpmm"
	StockOrcaWhirlpool StockDexType = "orca_whirlpool"
	StockMeteoraDlmm   StockDexType = "meteora_dlmm"
)

var stockDexPrograms = map[StockDexType]string{
	StockRaydiumClmm:   "CAMMCzo5YL8w4VFF8KVHrK22GGUsp5VTaW7grrKgrWqK",
	StockRaydiumCpmm:   "CPMMoo8L3F4NbTegBCKVNunggL7H1ZpdTHKxQB5qKP1C",
	StockOrcaWhirlpool: "whirLbMiicVdio4qvUfM5KAg6Ct8VwpYzGff3uctyCc",
	StockMeteoraDlmm:   "LBUZKhRxPF3XUpBCjp4YzTKgLccjZhTSDM9YuVaPwxo",
}

type StockToken struct {
	Mint          string `json:"mint"`
	Symbol        string `json:"symbol"`
	Decimals      uint8  `json:"decimals"`
	CatalogSource string `json:"catalog_source"`
}
type StockPool struct {
	Pool                  string          `json:"pool"`
	Mint                  string          `json:"mint"`
	QuoteMint             string          `json:"quote_mint"`
	DexType               StockDexType    `json:"dex_type"`
	CatalogFeeSnapshot    json.RawMessage `json:"catalog_fee_snapshot"`
	NominalSnapshotFeeBps *float64        `json:"nominal_snapshot_fee_bps"`
	LiquidSideReserveUSD  float64         `json:"liquid_side_reserve_usd"`
	Preferred             bool            `json:"preferred"`
}
type StockTokenCatalog struct {
	Tokens []StockToken `json:"tokens"`
	Pools  []StockPool  `json:"pools"`
}

// ScreenedStockCatalog explicitly opts into historical defaults and returns owned data.
func ScreenedStockCatalog() StockTokenCatalog {
	var catalog StockTokenCatalog
	if err := json.Unmarshal(bundledStockCatalog, &catalog); err != nil {
		panic(err)
	}
	return catalog
}
func (c StockTokenCatalog) PreferredPool(mint string) *StockPool {
	for _, pool := range c.Pools {
		if pool.Mint == mint {
			pool.CatalogFeeSnapshot = append(json.RawMessage(nil), pool.CatalogFeeSnapshot...)
			return &pool
		}
	}
	return nil
}

type StockRouteLeg struct {
	Pool       string  `json:"pool"`
	Program    string  `json:"program"`
	Protocol   string  `json:"protocol"`
	InputMint  *string `json:"input_mint"`
	OutputMint *string `json:"output_mint"`
}

// ConversionLegIndices preserves source order without pretending independent legs form a path.
func (c StockTokenCatalog) ConversionLegIndices(succeeded bool, legs []StockRouteLeg) []int {
	var result []int
	if !succeeded {
		return result
	}
	protocols := map[string]StockDexType{"RaydiumClmm": StockRaydiumClmm, "RaydiumCpmm": StockRaydiumCpmm, "OrcaWhirlpool": StockOrcaWhirlpool, "MeteoraDlmm": StockMeteoraDlmm}
	pools := make(map[string]StockPool, len(c.Pools))
	for _, pool := range c.Pools {
		pools[pool.Pool] = pool
	}
	for i, leg := range legs {
		pool, ok := pools[leg.Pool]
		dex := protocols[leg.Protocol]
		if !ok || dex != pool.DexType || leg.Program != stockDexPrograms[dex] || leg.InputMint == nil || leg.OutputMint == nil {
			continue
		}
		a, b := *leg.InputMint, *leg.OutputMint
		if (a == pool.Mint && b == pool.QuoteMint) || (a == pool.QuoteMint && b == pool.Mint) {
			result = append(result, i)
		}
	}
	return result
}
