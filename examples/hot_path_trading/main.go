package main

import (
	"fmt"
	"time"

	"github.com/0xfnzero/sol-trade-sdk-golang/examples/internal/exampleutil"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/hotpath"
	"github.com/gagliardetto/solana-go/rpc"
)

func main() {
	config := &hotpath.HotPathConfig{
		BlockhashRefreshInterval: 1500 * time.Millisecond,
		CacheTTL:                 4 * time.Second,
		EnablePrefetch:           true,
	}
	executor := hotpath.NewHotPathExecutor(rpc.New(exampleutil.RPCURL()), config)
	fmt.Println("Hot path executor prepared.")
	fmt.Println("Options:", hotpath.DefaultExecuteOptions())
	fmt.Println("Executor state ready:", executor.GetState().IsDataFresh())
}
