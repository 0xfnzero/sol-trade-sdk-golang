package main

import (
	"fmt"

	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/middleware"
)

func main() {
	manager := middleware.NewMiddlewareManager().
		AddMiddleware(middleware.NewValidationMiddleware(32, 1024)).
		AddMiddleware(&middleware.LoggingMiddleware{})
	instructions := []middleware.Instruction{{Data: []byte{1, 2, 3}}}
	processed, err := manager.ApplyMiddlewaresProcessProtocolInstructions(instructions, "PumpFun", true)
	if err != nil {
		panic(err)
	}
	fmt.Println("Middleware processed instructions:", len(processed))
}
