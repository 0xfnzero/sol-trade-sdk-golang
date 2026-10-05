package subscription

import (
	"errors"
	"github.com/gagliardetto/solana-go"
	"sort"
)

// Benchmark-only reference: full edge scan and base58 conversion inside sort comparisons.
// This models the previous search costs, not a historical release binary.
func visitCandidateRoutesScanReference(candidates []PoolTradeHint, input, output solana.PublicKey, maximumHops int, visit func([]PoolTradeHint) (bool, error)) error {
	if visit == nil {
		return errors.New("missing candidate visitor")
	}
	if maximumHops < 1 || maximumHops > 5 || input == output || input.IsZero() || output.IsZero() {
		return errors.New("invalid candidate route endpoints or hop limit")
	}
	pools := map[solana.PublicKey]PoolTradeHint{}
	for _, h := range candidates {
		if err := h.validate(); err != nil {
			return err
		}
		if old, ok := pools[h.Pool]; ok && !((old.InputMint == h.InputMint && old.OutputMint == h.OutputMint) || (old.InputMint == h.OutputMint && old.OutputMint == h.InputMint)) {
			return errors.New("conflicting candidate pool mints")
		}
		pools[h.Pool] = h
		if len(pools) > 64 {
			return errors.New("candidate search budget: maximum 64 pools")
		}
	}
	edges := []PoolTradeHint{}
	for _, h := range pools {
		edges = append(edges, h, PoolTradeHint{h.Pool, h.OutputMint, h.InputMint})
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].Pool == edges[j].Pool {
			return edges[i].OutputMint.String() < edges[j].OutputMint.String()
		}
		return edges[i].Pool.String() < edges[j].Pool.String()
	})
	type path struct {
		hints []PoolTradeHint
		mint  solana.PublicKey
		seen  map[solana.PublicKey]bool
	}
	frontier := []path{{nil, input, map[solana.PublicKey]bool{input: true}}}
	routes := 0
	for depth := 0; depth < maximumHops && len(frontier) > 0; depth++ {
		next := []path{}
		for _, p := range frontier {
			for _, h := range edges {
				if h.InputMint != p.mint || p.seen[h.OutputMint] {
					continue
				}
				reused := false
				for _, x := range p.hints {
					if x.Pool == h.Pool {
						reused = true
					}
				}
				if reused {
					continue
				}
				hints := append(append([]PoolTradeHint{}, p.hints...), h)
				if h.OutputMint == output {
					routes++
					if routes > 64 {
						return errors.New("candidate search budget: maximum 64 paths")
					}
					stop, err := visit(hints)
					if err != nil {
						return err
					}
					if stop {
						return nil
					}
				} else if depth+1 < maximumHops {
					if len(next) >= 4096 {
						return errors.New("candidate search budget exceeded")
					}
					seen := map[solana.PublicKey]bool{}
					for k := range p.seen {
						seen[k] = true
					}
					seen[h.OutputMint] = true
					next = append(next, path{hints, h.OutputMint, seen})
				}
			}
		}
		frontier = next
	}
	if routes == 0 {
		return errors.New("no connected candidate route")
	}
	return nil
}
