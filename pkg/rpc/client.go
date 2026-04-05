package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// ===== High Performance RPC Client =====

// Client is a high-performance Solana RPC client
type Client struct {
	endpoint      string
	httpClient    *http.Client
	headers       map[string]string
	requestID     int64
	timeout       time.Duration
	requests      int64
	errors        int64
	avgLatencyNs  int64
	totalLatencyNs int64
	mu            sync.RWMutex
}

// ClientOption is a functional option for the client
type ClientOption func(*Client)

// WithTimeout sets the request timeout
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.timeout = timeout
	}
}

// WithHeaders sets custom headers
func WithHeaders(headers map[string]string) ClientOption {
	return func(c *Client) {
		c.headers = headers
	}
}

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// NewClient creates a new RPC client
func NewClient(endpoint string, opts ...ClientOption) *Client {
	c := &Client{
		endpoint: endpoint,
		headers:  make(map[string]string),
		timeout:  30 * time.Second,
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.httpClient == nil {
		c.httpClient = &http.Client{
			Timeout: c.timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 20,
				IdleConnTimeout:     120 * time.Second,
				DisableCompression:  false,
			},
		}
	}
	return c
}

// ===== RPC Types =====

type rpcRequest struct {
	Jsonrpc string        `json:"jsonrpc"`
	ID      int64         `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params,omitempty"`
}

type rpcResponse struct {
	Jsonrpc string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ===== Core Methods =====

// Call makes a raw RPC call
func (c *Client) Call(ctx context.Context, method string, params ...interface{}) (json.RawMessage, error) {
	start := time.Now()
	atomic.AddInt64(&c.requests, 1)

	id := atomic.AddInt64(&c.requestID, 1)
	req := rpcRequest{
		Jsonrpc: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	body, err := json.Marshal(req)
	if err != nil {
		atomic.AddInt64(&c.errors, 1)
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, nil)
	if err != nil {
		atomic.AddInt64(&c.errors, 1)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Body = io.NopCloser(NewBytesReader(body))
	httpReq.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(NewBytesReader(body)), nil }
	httpReq.ContentLength = int64(len(body))
	httpReq.Header.Set("Content-Type", "application/json")

	c.mu.RLock()
	for k, v := range c.headers {
		httpReq.Header.Set(k, v)
	}
	c.mu.RUnlock()

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		atomic.AddInt64(&c.errors, 1)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	var rpcResp rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		atomic.AddInt64(&c.errors, 1)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	latency := time.Since(start).Nanoseconds()
	atomic.AddInt64(&c.totalLatencyNs, latency)

	if rpcResp.Error != nil {
		atomic.AddInt64(&c.errors, 1)
		return nil, &RPCCallError{Code: rpcResp.Error.Code, Message: rpcResp.Error.Message}
	}

	return rpcResp.Result, nil
}

// CallInto makes an RPC call and unmarshals the result
func (c *Client) CallInto(ctx context.Context, method string, result interface{}, params ...interface{}) error {
	raw, err := c.Call(ctx, method, params...)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, result); err != nil {
		return fmt.Errorf("failed to unmarshal result: %w", err)
	}
	return nil
}

// ===== Common RPC Methods =====

// GetBalance returns the balance of an account
func (c *Client) GetBalance(ctx context.Context, pubkey string, commitment string) (uint64, error) {
	var result struct {
		Value uint64 `json:"value"`
	}
	err := c.CallInto(ctx, "getBalance", &result, pubkey, map[string]string{"commitment": commitment})
	return result.Value, err
}

// GetAccountInfo returns account information
func (c *Client) GetAccountInfo(ctx context.Context, pubkey string, opts *AccountInfoConfig) (*AccountInfo, error) {
	params := []interface{}{pubkey}
	if opts != nil {
		params = append(params, opts)
	}

	var result struct {
		Value *AccountInfo `json:"value"`
	}
	err := c.CallInto(ctx, "getAccountInfo", &result, params...)
	return result.Value, err
}

// GetMultipleAccounts returns multiple accounts info
func (c *Client) GetMultipleAccounts(ctx context.Context, pubkeys []string, opts *AccountInfoConfig) ([]*AccountInfo, error) {
	params := []interface{}{pubkeys}
	if opts != nil {
		params = append(params, opts)
	}

	var result struct {
		Value []*AccountInfo `json:"value"`
	}
	err := c.CallInto(ctx, "getMultipleAccounts", &result, params...)
	return result.Value, err
}

// GetLatestBlockhash returns the latest blockhash
func (c *Client) GetLatestBlockhash(ctx context.Context, commitment string) (*BlockhashResult, error) {
	var result BlockhashResult
	err := c.CallInto(ctx, "getLatestBlockhash", &result, map[string]string{"commitment": commitment})
	return &result, err
}

// GetSignatureStatuses returns the status of signatures
func (c *Client) GetSignatureStatuses(ctx context.Context, signatures []string, searchTransactionHistory bool) ([]*SignatureStatus, error) {
	var result struct {
		Value []*SignatureStatus `json:"value"`
	}
	err := c.CallInto(ctx, "getSignatureStatuses", &result, signatures, map[string]bool{
		"searchTransactionHistory": searchTransactionHistory,
	})
	return result.Value, err
}

// SendTransaction sends a transaction
func (c *Client) SendTransaction(ctx context.Context, tx []byte, opts *SendOptions) (string, error) {
	params := []interface{}{tx}
	if opts != nil {
		params = append(params, opts)
	}

	var result string
	err := c.CallInto(ctx, "sendTransaction", &result, params...)
	return result, err
}

// SimulateTransaction simulates a transaction
func (c *Client) SimulateTransaction(ctx context.Context, tx []byte, opts *SimulateOptions) (*SimulateResult, error) {
	params := []interface{}{tx}
	if opts != nil {
		params = append(params, opts)
	}

	var result struct {
		Value SimulateResult `json:"value"`
	}
	err := c.CallInto(ctx, "simulateTransaction", &result, params...)
	return &result.Value, err
}

// GetTokenAccountsByOwner returns token accounts by owner
func (c *Client) GetTokenAccountsByOwner(ctx context.Context, owner string, filter *TokenAccountFilter, opts *AccountInfoConfig) ([]TokenAccount, error) {
	var result struct {
		Value []TokenAccount `json:"value"`
	}
	params := []interface{}{owner, filter}
	if opts != nil {
		params = append(params, opts)
	}
	err := c.CallInto(ctx, "getTokenAccountsByOwner", &result, params...)
	return result.Value, err
}

// ===== Types =====

// AccountInfo represents account information
type AccountInfo struct {
	Lamports  uint64 `json:"lamports"`
	Data      []byte `json:"data"`
	Owner     string `json:"owner"`
	Executable bool   `json:"executable"`
	RentEpoch uint64 `json:"rentEpoch"`
}

// AccountInfoConfig represents account info config
type AccountInfoConfig struct {
	Encoding       string `json:"encoding,omitempty"`
	DataSlice      *DataSlice `json:"dataSlice,omitempty"`
	MinContextSlot uint64 `json:"minContextSlot,omitempty"`
}

// DataSlice represents data slice options
type DataSlice struct {
	Offset uint64 `json:"offset"`
	Length uint64 `json:"length"`
}

// BlockhashResult represents a blockhash result
type BlockhashResult struct {
	Blockhash          string `json:"blockhash"`
	LastValidBlockHeight uint64 `json:"lastValidBlockHeight"`
}

// SignatureStatus represents signature status
type SignatureStatus struct {
	Slot              uint64          `json:"slot"`
	Confirmations     *uint64         `json:"confirmations,omitempty"`
	Err               json.RawMessage `json:"err,omitempty"`
	ConfirmationStatus string         `json:"confirmationStatus"`
}

// SendOptions represents send transaction options
type SendOptions struct {
	SkipPreflight       bool   `json:"skipPreflight,omitempty"`
	PreflightCommitment string `json:"preflightCommitment,omitempty"`
	MaxRetries          uint64 `json:"maxRetries,omitempty"`
	MinContextSlot      uint64 `json:"minContextSlot,omitempty"`
}

// SimulateOptions represents simulate transaction options
type SimulateOptions struct {
	SigVerify          bool     `json:"sigVerify,omitempty"`
	ReplaceRecentBlockhash bool `json:"replaceRecentBlockhash,omitempty"`
	Commitment         string   `json:"commitment,omitempty"`
	Accounts           *SimulateAccounts `json:"accounts,omitempty"`
}

// SimulateAccounts represents accounts for simulation
type SimulateAccounts struct {
	Encoding string `json:"encoding"`
	Addresses []string `json:"addresses"`
}

// SimulateResult represents simulation result
type SimulateResult struct {
	Err           json.RawMessage `json:"err,omitempty"`
	Accounts      []*AccountInfo  `json:"accounts,omitempty"`
	UnitsConsumed uint64          `json:"unitsConsumed"`
	ReturnData    *ReturnData     `json:"returnData,omitempty"`
}

// ReturnData represents return data from simulation
type ReturnData struct {
	Data []byte `json:"data"`
	ProgramID string `json:"programId"`
}

// TokenAccount represents a token account
type TokenAccount struct {
	Pubkey  string       `json:"pubkey"`
	Account *AccountInfo `json:"account"`
}

// TokenAccountFilter represents token account filter
type TokenAccountFilter struct {
	Mint   string `json:"mint,omitempty"`
	Program string `json:"programId,omitempty"`
}

// RPCCallError represents an RPC call error
type RPCCallError struct {
	Code    int
	Message string
}

func (e *RPCCallError) Error() string {
	return fmt.Sprintf("RPC error %d: %s", e.Code, e.Message)
}

// ===== Stats =====

// Stats returns client statistics
func (c *Client) Stats() (requests, errors int64, avgLatency time.Duration) {
	requests = atomic.LoadInt64(&c.requests)
	errors = atomic.LoadInt64(&c.errors)
	totalNs := atomic.LoadInt64(&c.totalLatencyNs)
	if requests > 0 {
		avgLatency = time.Duration(totalNs/requests) * time.Nanosecond
	}
	return
}

// ===== Helpers =====

// BytesReader is an io.Reader for a byte slice
type BytesReader struct {
	data []byte
	pos  int
}

// NewBytesReader creates a new bytes reader
func NewBytesReader(data []byte) *BytesReader {
	return &BytesReader{data: data}
}

func (r *BytesReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
