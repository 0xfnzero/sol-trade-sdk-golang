// Middleware System Example
//
// This example demonstrates how to use the middleware system
// to modify, add, or remove instructions before transaction execution.

package main

import (
	"fmt"

	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/middleware"
)

func main() {
	fmt.Println("Middleware System Example")
	fmt.Println("This example demonstrates how to use the middleware system")

	// Create middleware manager
	manager := middleware.NewMiddlewareManager()

	// Add validation middleware
	validationMiddleware := &middleware.ValidationMiddleware{
		MaxInstructions: 100,
		MaxDataSize:     10000,
	}
	manager.AddMiddleware(validationMiddleware)

	// Add logging middleware
	loggingMiddleware := &middleware.LoggingMiddleware{}
	manager.AddMiddleware(loggingMiddleware)

	fmt.Println("Middleware manager created with validation and logging middlewares")

	// In a real scenario, you would apply middlewares to instructions:
	// processed, err := manager.ApplyMiddlewaresProcessProtocolInstructions(
	//     instructions, "PumpFun", true,
	// )

	fmt.Println("Middleware system example completed!")
}
