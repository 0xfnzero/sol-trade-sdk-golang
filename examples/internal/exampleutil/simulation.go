package exampleutil

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func ValidateSimulationResponse(body []byte) error {
	var r struct {
		Error  json.RawMessage `json:"error"`
		Result *struct {
			Value *struct {
				Err json.RawMessage `json:"err"`
			} `json:"value"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return errors.New("invalid simulation JSON")
	}
	if len(r.Error) > 0 && string(r.Error) != "null" {
		return errors.New("simulation RPC error")
	}
	if r.Result == nil || r.Result.Value == nil || len(r.Result.Value.Err) == 0 {
		return errors.New("simulation response missing execution result")
	}
	if string(r.Result.Value.Err) != "null" {
		return errors.New("simulation failed; inspect saved logs")
	}
	return nil
}

// Only the explicitly requested simulation calls RPC; nothing is submitted.
func SimulateIfRequested(args []string, wire []byte, slot uint64) error {
	flags := flag.NewFlagSet("simulation", flag.ContinueOnError)
	simulate := flags.Bool("simulate", false, "simulate without sending")
	output := flags.String("simulation-out", "", "save original wire and response for parser")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected simulation argument")
	}
	if !*simulate {
		if *output != "" {
			return errors.New("--simulation-out requires --simulate")
		}
		return nil
	}
	endpoint := os.Getenv("RPC_URL")
	if endpoint == "" {
		endpoint = "https://api.mainnet-beta.solana.com"
	}
	encoded := base64.StdEncoding.EncodeToString(wire)
	payload, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "simulateTransaction", "params": []any{encoded, map[string]any{"encoding": "base64", "sigVerify": false, "replaceRecentBlockhash": true, "innerInstructions": true, "commitment": "confirmed", "minContextSlot": slot}}})
	if err != nil {
		return err
	}
	client := http.Client{Timeout: 30 * time.Second}
	response, err := client.Post(endpoint, "application/json", bytes.NewReader(payload))
	if err != nil {
		return errors.New("simulation transport failed")
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 16<<20))
	if err != nil {
		return errors.New("simulation response read failed")
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("simulation HTTP %d", response.StatusCode)
	}
	if !json.Valid(body) {
		return errors.New("invalid simulation JSON")
	}
	if *output != "" {
		evidence, err := json.MarshalIndent(map[string]any{"wire": encoded, "response": json.RawMessage(body)}, "", "  ")
		if err != nil {
			return err
		}
		if err = os.WriteFile(*output, append(evidence, '\n'), 0600); err != nil {
			return err
		}
	}
	fmt.Println(string(body))
	return ValidateSimulationResponse(body)
}
