package swqos

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	soltradesdk "github.com/your-org/sol-trade-sdk-go"
	"github.com/gagliardetto/solana-go"
)

// ===== Types =====

// TradeType represents buy or sell operation
type TradeType int

const (
	TradeTypeCreate TradeType = iota
	TradeTypeCreateAndBuy
	TradeTypeBuy
	TradeTypeSell
)

func (t TradeType) String() string {
	return [...]string{"Create", "CreateAndBuy", "Buy", "Sell"}[t]
}

// SwqosType represents the type of SWQOS service
type SwqosType int

const (
	SwqosTypeJito SwqosType = iota
	SwqosTypeNextBlock
	SwqosTypeZeroSlot
	SwqosTypeTemporal
	SwqosTypeBloxroute
	SwqosTypeNode1
	SwqosTypeFlashBlock
	SwqosTypeBlockRazor
	SwqosTypeAstralane
	SwqosTypeStellium
	SwqosTypeLightspeed
	SwqosTypeSoyas
	SwqosTypeSpeedlanding
	SwqosTypeHelius
	SwqosTypeDefault
)

func (s SwqosType) String() string {
	return [...]string{
		"Jito", "NextBlock", "ZeroSlot", "Temporal", "Bloxroute",
		"Node1", "FlashBlock", "BlockRazor", "Astralane", "Stellium",
		"Lightspeed", "Soyas", "Speedlanding", "Helius", "Default",
	}[s]
}

// GetAllSwqosTypes returns all SWQOS types
func GetAllSwqosTypes() []SwqosType {
	return []SwqosType{
		SwqosTypeJito, SwqosTypeNextBlock, SwqosTypeZeroSlot, SwqosTypeTemporal,
		SwqosTypeBloxroute, SwqosTypeNode1, SwqosTypeFlashBlock, SwqosTypeBlockRazor,
		SwqosTypeAstralane, SwqosTypeStellium, SwqosTypeLightspeed, SwqosTypeSoyas,
		SwqosTypeSpeedlanding, SwqosTypeHelius, SwqosTypeDefault,
	}
}

// SwqosRegion represents SWQOS service region
type SwqosRegion int

const (
	SwqosRegionNewYork SwqosRegion = iota
	SwqosRegionFrankfurt
	SwqosRegionAmsterdam
	SwqosRegionSLC
	SwqosRegionTokyo
	SwqosRegionLondon
	SwqosRegionLosAngeles
	SwqosRegionDefault
)

// SwqosTransport represents the transport type
type SwqosTransport int

const (
	SwqosTransportHTTP SwqosTransport = iota
	SwqosTransportGRPC
	SwqosTransportQUIC
)

// ===== Constants =====

// Minimum tips in SOL for each provider
const (
	MinTipJito        = 0.001
	MinTipBloxroute   = 0.0003
	MinTipZeroSlot    = 0.0001
	MinTipTemporal    = 0.0001
	MinTipFlashBlock  = 0.0001
	MinTipBlockRazor  = 0.0001
	MinTipNode1       = 0.0001
	MinTipAstralane   = 0.0001
	MinTipHelius      = 0.000005 // SWQOS-only mode
	MinTipDefault     = 0.0
)

// Endpoints for each provider by region
var (
	JitoEndpoints = map[SwqosRegion]string{
		SwqosRegionNewYork:   "amsterdam.mainnet.block-engine.jito.wtf",
		SwqosRegionFrankfurt: "frankfurt.mainnet.block-engine.jito.wtf",
		SwqosRegionAmsterdam: "amsterdam.mainnet.block-engine.jito.wtf",
		SwqosRegionTokyo:     "tokyo.mainnet.block-engine.jito.wtf",
	}
)

// ===== Interfaces =====

// SwqosClient defines the interface for SWQOS clients
type SwqosClient interface {
	SendTransaction(ctx context.Context, tradeType TradeType, transaction []byte, waitConfirmation bool) (solana.Signature, error)
	SendTransactions(ctx context.Context, tradeType TradeType, transactions [][]byte, waitConfirmation bool) ([]solana.Signature, error)
	GetTipAccount() string
	GetSwqosType() SwqosType
	MinTipSol() float64
}

// TradeError represents a trade error
type TradeError struct {
	Code       uint32
	Message    string
	Instruction *uint8
}

func (e *TradeError) Error() string {
	return e.Message
}

// ===== HTTP Client =====

var (
	httpClient     *http.Client
	httpClientOnce sync.Once
)

func getHTTPClient() *http.Client {
	httpClientOnce.Do(func() {
		httpClient = &http.Client{
			Timeout: 3 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 4,
				IdleConnTimeout:     300 * time.Second,
			},
		}
	})
	return httpClient
}

// ===== Jito Client =====

// JitoClient represents a Jito SWQOS client
type JitoClient struct {
	rpcURL    string
	endpoint  string
	authToken string
	tipAccount string
}

// NewJitoClient creates a new Jito client
func NewJitoClient(rpcURL, endpoint, authToken string) *JitoClient {
	return &JitoClient{
		rpcURL:     rpcURL,
		endpoint:   endpoint,
		authToken:  authToken,
		tipAccount: "96gYZGLnJYVFmbjzopPSU6QiEV5fGqZNyN9nmBUvrNei",
	}
}

// SendTransaction sends a transaction via Jito
func (c *JitoClient) SendTransaction(ctx context.Context, tradeType TradeType, transaction []byte, waitConfirmation bool) (solana.Signature, error) {
	encoded := base64.StdEncoding.EncodeToString(transaction)

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "sendTransaction",
		"params": []interface{}{
			encoded,
			map[string]interface{}{
				"encoding": "base64",
			},
		},
	}

	jsonData, _ := json.Marshal(payload)
	url := fmt.Sprintf("https://%s/api/v1/bundles", c.endpoint)

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return solana.Signature{}, err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.authToken != "" {
		req.Header.Set("X-Jito-Auth-Token", c.authToken)
	}

	resp, err := getHTTPClient().Do(req)
	if err != nil {
		return solana.Signature{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Result string `json:"result"`
		Error  struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return solana.Signature{}, err
	}

	if result.Error.Message != "" {
		return solana.Signature{}, &TradeError{Code: 500, Message: result.Error.Message}
	}

	sig, err := solana.SignatureFromBase58(result.Result)
	if err != nil {
		return solana.Signature{}, err
	}

	return sig, nil
}

// SendTransactions sends multiple transactions via Jito
func (c *JitoClient) SendTransactions(ctx context.Context, tradeType TradeType, transactions [][]byte, waitConfirmation bool) ([]solana.Signature, error) {
	sigs := make([]solana.Signature, len(transactions))
	for i, tx := range transactions {
		sig, err := c.SendTransaction(ctx, tradeType, tx, waitConfirmation)
		if err != nil {
			return sigs, err
		}
		sigs[i] = sig
	}
	return sigs, nil
}

// GetTipAccount returns the Jito tip account
func (c *JitoClient) GetTipAccount() string {
	return c.tipAccount
}

// GetSwqosType returns Jito type
func (c *JitoClient) GetSwqosType() SwqosType {
	return SwqosTypeJito
}

// MinTipSol returns minimum tip for Jito
func (c *JitoClient) MinTipSol() float64 {
	return MinTipJito
}

// ===== Bloxroute Client =====

// BloxrouteClient represents a Bloxroute SWQOS client
type BloxrouteClient struct {
	rpcURL    string
	endpoint  string
	authToken string
	tipAccount string
}

// NewBloxrouteClient creates a new Bloxroute client
func NewBloxrouteClient(rpcURL, endpoint, authToken string) *BloxrouteClient {
	return &BloxrouteClient{
		rpcURL:     rpcURL,
		endpoint:   endpoint,
		authToken:  authToken,
		tipAccount: "HWeXY6GuqP3i2vMPUgwt4XPq5LqSvdkfF3R6dQ5ciPfo",
	}
}

// SendTransaction sends a transaction via Bloxroute
func (c *BloxrouteClient) SendTransaction(ctx context.Context, tradeType TradeType, transaction []byte, waitConfirmation bool) (solana.Signature, error) {
	encoded := base64.StdEncoding.EncodeToString(transaction)

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "sendTransaction",
		"params": []interface{}{
			encoded,
		},
	}

	jsonData, _ := json.Marshal(payload)
	url := fmt.Sprintf("https://%s/api/v2/submit", c.endpoint)

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return solana.Signature{}, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", c.authToken)

	resp, err := getHTTPClient().Do(req)
	if err != nil {
		return solana.Signature{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Signature string `json:"signature"`
		Reason    string `json:"reason"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return solana.Signature{}, err
	}

	if result.Reason != "" {
		return solana.Signature{}, &TradeError{Code: 500, Message: result.Reason}
	}

	sig, err := solana.SignatureFromBase58(result.Signature)
	if err != nil {
		return solana.Signature{}, err
	}

	return sig, nil
}

// SendTransactions sends multiple transactions via Bloxroute
func (c *BloxrouteClient) SendTransactions(ctx context.Context, tradeType TradeType, transactions [][]byte, waitConfirmation bool) ([]solana.Signature, error) {
	sigs := make([]solana.Signature, len(transactions))
	for i, tx := range transactions {
		sig, err := c.SendTransaction(ctx, tradeType, tx, waitConfirmation)
		if err != nil {
			return sigs, err
		}
		sigs[i] = sig
	}
	return sigs, nil
}

// GetTipAccount returns the Bloxroute tip account
func (c *BloxrouteClient) GetTipAccount() string {
	return c.tipAccount
}

// GetSwqosType returns Bloxroute type
func (c *BloxrouteClient) GetSwqosType() SwqosType {
	return SwqosTypeBloxroute
}

// MinTipSol returns minimum tip for Bloxroute
func (c *BloxrouteClient) MinTipSol() float64 {
	return MinTipBloxroute
}

// ===== ZeroSlot Client =====

// ZeroSlotClient represents a ZeroSlot SWQOS client
type ZeroSlotClient struct {
	rpcURL    string
	endpoint  string
	authToken string
	tipAccount string
}

// NewZeroSlotClient creates a new ZeroSlot client
func NewZeroSlotClient(rpcURL, endpoint, authToken string) *ZeroSlotClient {
	return &ZeroSlotClient{
		rpcURL:     rpcURL,
		endpoint:   endpoint,
		authToken:  authToken,
		tipAccount: "zeroslotH4gNdW3DyUr3QYjE3QiPYq78mi4jh7U3YyHY",
	}
}

// SendTransaction sends a transaction via ZeroSlot
func (c *ZeroSlotClient) SendTransaction(ctx context.Context, tradeType TradeType, transaction []byte, waitConfirmation bool) (solana.Signature, error) {
	encoded := base64.StdEncoding.EncodeToString(transaction)

	payload := map[string]interface{}{
		"transaction": encoded,
	}

	jsonData, _ := json.Marshal(payload)
	url := fmt.Sprintf("https://%s/api/v1/submit", c.endpoint)

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return solana.Signature{}, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.authToken)

	resp, err := getHTTPClient().Do(req)
	if err != nil {
		return solana.Signature{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Signature string `json:"signature"`
		Error     string `json:"error"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return solana.Signature{}, err
	}

	if result.Error != "" {
		return solana.Signature{}, &TradeError{Code: 500, Message: result.Error}
	}

	sig, err := solana.SignatureFromBase58(result.Signature)
	if err != nil {
		return solana.Signature{}, err
	}

	return sig, nil
}

// SendTransactions sends multiple transactions via ZeroSlot
func (c *ZeroSlotClient) SendTransactions(ctx context.Context, tradeType TradeType, transactions [][]byte, waitConfirmation bool) ([]solana.Signature, error) {
	sigs := make([]solana.Signature, len(transactions))
	for i, tx := range transactions {
		sig, err := c.SendTransaction(ctx, tradeType, tx, waitConfirmation)
		if err != nil {
			return sigs, err
		}
		sigs[i] = sig
	}
	return sigs, nil
}

// GetTipAccount returns the ZeroSlot tip account
func (c *ZeroSlotClient) GetTipAccount() string {
	return c.tipAccount
}

// GetSwqosType returns ZeroSlot type
func (c *ZeroSlotClient) GetSwqosType() SwqosType {
	return SwqosTypeZeroSlot
}

// MinTipSol returns minimum tip for ZeroSlot
func (c *ZeroSlotClient) MinTipSol() float64 {
	return MinTipZeroSlot
}

// ===== Temporal Client =====

// TemporalClient represents a Temporal SWQOS client
type TemporalClient struct {
	rpcURL    string
	endpoint  string
	authToken string
	tipAccount string
}

// NewTemporalClient creates a new Temporal client
func NewTemporalClient(rpcURL, endpoint, authToken string) *TemporalClient {
	return &TemporalClient{
		rpcURL:     rpcURL,
		endpoint:   endpoint,
		authToken:  authToken,
		tipAccount: "temporalGxiRP8dLKPhUT6vJ6Qnq1RmqNGW8mVu8mPTwogbNX7j",
	}
}

// SendTransaction sends a transaction via Temporal
func (c *TemporalClient) SendTransaction(ctx context.Context, tradeType TradeType, transaction []byte, waitConfirmation bool) (solana.Signature, error) {
	encoded := base64.StdEncoding.EncodeToString(transaction)

	payload := map[string]interface{}{
		"transaction": encoded,
	}

	jsonData, _ := json.Marshal(payload)
	url := fmt.Sprintf("https://%s/api/v1/submit", c.endpoint)

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return solana.Signature{}, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", c.authToken)

	resp, err := getHTTPClient().Do(req)
	if err != nil {
		return solana.Signature{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Signature string `json:"signature"`
		Error     string `json:"error"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return solana.Signature{}, err
	}

	if result.Error != "" {
		return solana.Signature{}, &TradeError{Code: 500, Message: result.Error}
	}

	sig, err := solana.SignatureFromBase58(result.Signature)
	if err != nil {
		return solana.Signature{}, err
	}

	return sig, nil
}

// SendTransactions sends multiple transactions via Temporal
func (c *TemporalClient) SendTransactions(ctx context.Context, tradeType TradeType, transactions [][]byte, waitConfirmation bool) ([]solana.Signature, error) {
	sigs := make([]solana.Signature, len(transactions))
	for i, tx := range transactions {
		sig, err := c.SendTransaction(ctx, tradeType, tx, waitConfirmation)
		if err != nil {
			return sigs, err
		}
		sigs[i] = sig
	}
	return sigs, nil
}

// GetTipAccount returns the Temporal tip account
func (c *TemporalClient) GetTipAccount() string {
	return c.tipAccount
}

// GetSwqosType returns Temporal type
func (c *TemporalClient) GetSwqosType() SwqosType {
	return SwqosTypeTemporal
}

// MinTipSol returns minimum tip for Temporal
func (c *TemporalClient) MinTipSol() float64 {
	return MinTipTemporal
}

// ===== FlashBlock Client =====

// FlashBlockClient represents a FlashBlock SWQOS client
type FlashBlockClient struct {
	rpcURL    string
	endpoint  string
	authToken string
	tipAccount string
}

// NewFlashBlockClient creates a new FlashBlock client
func NewFlashBlockClient(rpcURL, endpoint, authToken string) *FlashBlockClient {
	return &FlashBlockClient{
		rpcURL:     rpcURL,
		endpoint:   endpoint,
		authToken:  authToken,
		tipAccount: "flashblockHjE4frLuq8iFzboHy5AW8VZMo7mDhjt4VhV",
	}
}

// SendTransaction sends a transaction via FlashBlock
func (c *FlashBlockClient) SendTransaction(ctx context.Context, tradeType TradeType, transaction []byte, waitConfirmation bool) (solana.Signature, error) {
	encoded := base64.StdEncoding.EncodeToString(transaction)

	payload := map[string]interface{}{
		"transaction": encoded,
	}

	jsonData, _ := json.Marshal(payload)
	url := fmt.Sprintf("https://%s/api/v1/submit", c.endpoint)

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return solana.Signature{}, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.authToken)

	resp, err := getHTTPClient().Do(req)
	if err != nil {
		return solana.Signature{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Signature string `json:"signature"`
		Error     string `json:"error"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return solana.Signature{}, err
	}

	if result.Error != "" {
		return solana.Signature{}, &TradeError{Code: 500, Message: result.Error}
	}

	sig, err := solana.SignatureFromBase58(result.Signature)
	if err != nil {
		return solana.Signature{}, err
	}

	return sig, nil
}

// SendTransactions sends multiple transactions via FlashBlock
func (c *FlashBlockClient) SendTransactions(ctx context.Context, tradeType TradeType, transactions [][]byte, waitConfirmation bool) ([]solana.Signature, error) {
	sigs := make([]solana.Signature, len(transactions))
	for i, tx := range transactions {
		sig, err := c.SendTransaction(ctx, tradeType, tx, waitConfirmation)
		if err != nil {
			return sigs, err
		}
		sigs[i] = sig
	}
	return sigs, nil
}

// GetTipAccount returns the FlashBlock tip account
func (c *FlashBlockClient) GetTipAccount() string {
	return c.tipAccount
}

// GetSwqosType returns FlashBlock type
func (c *FlashBlockClient) GetSwqosType() SwqosType {
	return SwqosTypeFlashBlock
}

// MinTipSol returns minimum tip for FlashBlock
func (c *FlashBlockClient) MinTipSol() float64 {
	return MinTipFlashBlock
}

// ===== Helius Client =====

// HeliusClient represents a Helius SWQOS client
type HeliusClient struct {
	rpcURL     string
	endpoint   string
	apiKey     string
	swqosOnly  bool
	tipAccount string
}

// NewHeliusClient creates a new Helius client
func NewHeliusClient(rpcURL, endpoint string, apiKey *string, swqosOnly bool) *HeliusClient {
	key := ""
	if apiKey != nil {
		key = *apiKey
	}
	return &HeliusClient{
		rpcURL:     rpcURL,
		endpoint:   endpoint,
		apiKey:     key,
		swqosOnly:  swqosOnly,
		tipAccount: "heliusH4gNdW3DyUr3QYjE3QiPYq78mi4jh7U3YyHY",
	}
}

// SendTransaction sends a transaction via Helius
func (c *HeliusClient) SendTransaction(ctx context.Context, tradeType TradeType, transaction []byte, waitConfirmation bool) (solana.Signature, error) {
	encoded := base64.StdEncoding.EncodeToString(transaction)

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "sendTransaction",
		"params": []interface{}{
			encoded,
			map[string]interface{}{
				"encoding": "base64",
			},
		},
	}

	jsonData, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, strings.NewReader(string(jsonData)))
	if err != nil {
		return solana.Signature{}, err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := getHTTPClient().Do(req)
	if err != nil {
		return solana.Signature{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Result string `json:"result"`
		Error  struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return solana.Signature{}, err
	}

	if result.Error.Message != "" {
		return solana.Signature{}, &TradeError{Code: 500, Message: result.Error.Message}
	}

	sig, err := solana.SignatureFromBase58(result.Result)
	if err != nil {
		return solana.Signature{}, err
	}

	return sig, nil
}

// SendTransactions sends multiple transactions via Helius
func (c *HeliusClient) SendTransactions(ctx context.Context, tradeType TradeType, transactions [][]byte, waitConfirmation bool) ([]solana.Signature, error) {
	sigs := make([]solana.Signature, len(transactions))
	for i, tx := range transactions {
		sig, err := c.SendTransaction(ctx, tradeType, tx, waitConfirmation)
		if err != nil {
			return sigs, err
		}
		sigs[i] = sig
	}
	return sigs, nil
}

// GetTipAccount returns the Helius tip account
func (c *HeliusClient) GetTipAccount() string {
	return c.tipAccount
}

// GetSwqosType returns Helius type
func (c *HeliusClient) GetSwqosType() SwqosType {
	return SwqosTypeHelius
}

// MinTipSol returns minimum tip for Helius
func (c *HeliusClient) MinTipSol() float64 {
	if c.swqosOnly {
		return MinTipHelius
	}
	return 0.0002
}

// ===== BlockRazor Client =====

// BlockRazorClient represents a BlockRazor SWQOS client
type BlockRazorClient struct {
	rpcURL        string
	endpoint      string
	authToken     string
	mevProtection bool
	tipAccount    string
}

// NewBlockRazorClient creates a new BlockRazor client
func NewBlockRazorClient(rpcURL, endpoint, authToken string, mevProtection bool) *BlockRazorClient {
	return &BlockRazorClient{
		rpcURL:        rpcURL,
		endpoint:      endpoint,
		authToken:     authToken,
		mevProtection: mevProtection,
		tipAccount:    "blockrazorH4gNdW3DyUr3QYjE3QiPYq78mi4jh7U3YyHY",
	}
}

// SendTransaction sends a transaction via BlockRazor
func (c *BlockRazorClient) SendTransaction(ctx context.Context, tradeType TradeType, transaction []byte, waitConfirmation bool) (solana.Signature, error) {
	encoded := base64.StdEncoding.EncodeToString(transaction)

	payload := map[string]interface{}{
		"transaction": encoded,
	}

	jsonData, _ := json.Marshal(payload)

	mode := "fast"
	if c.mevProtection {
		mode = "sandwichMitigation"
	}
	url := fmt.Sprintf("https://%s/api/v1/submit?mode=%s", c.endpoint, mode)

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return solana.Signature{}, err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.authToken != "" {
		req.Header.Set("X-API-Key", c.authToken)
	}

	resp, err := getHTTPClient().Do(req)
	if err != nil {
		return solana.Signature{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Signature string `json:"signature"`
		Error     string `json:"error"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return solana.Signature{}, err
	}

	if result.Error != "" {
		return solana.Signature{}, &TradeError{Code: 500, Message: result.Error}
	}

	sig, err := solana.SignatureFromBase58(result.Signature)
	if err != nil {
		return solana.Signature{}, err
	}

	return sig, nil
}

// SendTransactions sends multiple transactions via BlockRazor
func (c *BlockRazorClient) SendTransactions(ctx context.Context, tradeType TradeType, transactions [][]byte, waitConfirmation bool) ([]solana.Signature, error) {
	sigs := make([]solana.Signature, len(transactions))
	for i, tx := range transactions {
		sig, err := c.SendTransaction(ctx, tradeType, tx, waitConfirmation)
		if err != nil {
			return sigs, err
		}
		sigs[i] = sig
	}
	return sigs, nil
}

// GetTipAccount returns the BlockRazor tip account
func (c *BlockRazorClient) GetTipAccount() string { return c.tipAccount }

// GetSwqosType returns BlockRazor type
func (c *BlockRazorClient) GetSwqosType() SwqosType { return SwqosTypeBlockRazor }

// MinTipSol returns minimum tip for BlockRazor
func (c *BlockRazorClient) MinTipSol() float64 { return MinTipBlockRazor }

// ===== Astralane Client =====

// AstralaneClient represents an Astralane SWQOS client
type AstralaneClient struct {
	rpcURL     string
	endpoint   string
	authToken  string
	tipAccount string
}

// NewAstralaneClient creates a new Astralane client
func NewAstralaneClient(rpcURL, endpoint, authToken string) *AstralaneClient {
	return &AstralaneClient{
		rpcURL:     rpcURL,
		endpoint:   endpoint,
		authToken:  authToken,
		tipAccount: "astralaneH4gNdW3DyUr3QYjE3QiPYq78mi4jh7U3YyHY",
	}
}

// SendTransaction sends a transaction via Astralane
func (c *AstralaneClient) SendTransaction(ctx context.Context, tradeType TradeType, transaction []byte, waitConfirmation bool) (solana.Signature, error) {
	encoded := base64.StdEncoding.EncodeToString(transaction)

	payload := map[string]interface{}{
		"transaction": encoded,
	}

	jsonData, _ := json.Marshal(payload)
	url := fmt.Sprintf("https://%s/api/v1/submit", c.endpoint)

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return solana.Signature{}, err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	resp, err := getHTTPClient().Do(req)
	if err != nil {
		return solana.Signature{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Signature string `json:"signature"`
		Error     string `json:"error"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return solana.Signature{}, err
	}

	if result.Error != "" {
		return solana.Signature{}, &TradeError{Code: 500, Message: result.Error}
	}

	sig, err := solana.SignatureFromBase58(result.Signature)
	if err != nil {
		return solana.Signature{}, err
	}

	return sig, nil
}

// SendTransactions sends multiple transactions via Astralane
func (c *AstralaneClient) SendTransactions(ctx context.Context, tradeType TradeType, transactions [][]byte, waitConfirmation bool) ([]solana.Signature, error) {
	sigs := make([]solana.Signature, len(transactions))
	for i, tx := range transactions {
		sig, err := c.SendTransaction(ctx, tradeType, tx, waitConfirmation)
		if err != nil {
			return sigs, err
		}
		sigs[i] = sig
	}
	return sigs, nil
}

// GetTipAccount returns the Astralane tip account
func (c *AstralaneClient) GetTipAccount() string { return c.tipAccount }

// GetSwqosType returns Astralane type
func (c *AstralaneClient) GetSwqosType() SwqosType { return SwqosTypeAstralane }

// MinTipSol returns minimum tip for Astralane
func (c *AstralaneClient) MinTipSol() float64 { return MinTipAstralane }

// ===== Default RPC Client =====

// DefaultClient represents a default RPC client
type DefaultClient struct {
	rpcURL string
}

// NewDefaultClient creates a new default client
func NewDefaultClient(rpcURL string) *DefaultClient {
	return &DefaultClient{rpcURL: rpcURL}
}

// SendTransaction sends a transaction via default RPC
func (c *DefaultClient) SendTransaction(ctx context.Context, tradeType TradeType, transaction []byte, waitConfirmation bool) (solana.Signature, error) {
	encoded := base64.StdEncoding.EncodeToString(transaction)

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "sendTransaction",
		"params": []interface{}{
			encoded,
			map[string]interface{}{
				"encoding": "base64",
			},
		},
	}

	jsonData, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", c.rpcURL, strings.NewReader(string(jsonData)))
	if err != nil {
		return solana.Signature{}, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := getHTTPClient().Do(req)
	if err != nil {
		return solana.Signature{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Result string `json:"result"`
		Error  struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return solana.Signature{}, err
	}

	if result.Error.Message != "" {
		return solana.Signature{}, &TradeError{Code: 500, Message: result.Error.Message}
	}

	sig, err := solana.SignatureFromBase58(result.Result)
	if err != nil {
		return solana.Signature{}, err
	}

	return sig, nil
}

// SendTransactions sends multiple transactions via default RPC
func (c *DefaultClient) SendTransactions(ctx context.Context, tradeType TradeType, transactions [][]byte, waitConfirmation bool) ([]solana.Signature, error) {
	sigs := make([]solana.Signature, len(transactions))
	for i, tx := range transactions {
		sig, err := c.SendTransaction(ctx, tradeType, tx, waitConfirmation)
		if err != nil {
			return sigs, err
		}
		sigs[i] = sig
	}
	return sigs, nil
}

// GetTipAccount returns empty string for default RPC
func (c *DefaultClient) GetTipAccount() string {
	return ""
}

// GetSwqosType returns Default type
func (c *DefaultClient) GetSwqosType() SwqosType {
	return SwqosTypeDefault
}

// MinTipSol returns 0 for default RPC
func (c *DefaultClient) MinTipSol() float64 {
	return MinTipDefault
}

// ===== Client Factory =====

// ClientFactory creates SWQOS clients based on config
type ClientFactory struct{}

// CreateClient creates a SWQOS client from config
func (f *ClientFactory) CreateClient(config soltradesdk.SwqosConfig, rpcURL string) (SwqosClient, error) {
	switch config.Type {
	case SwqosTypeJito:
		endpoint := JitoEndpoints[config.Region]
		if config.CustomURL != "" {
			endpoint = config.CustomURL
		}
		return NewJitoClient(rpcURL, endpoint, config.APIKey), nil

	case SwqosTypeBloxroute:
		return NewBloxrouteClient(rpcURL, config.CustomURL, config.APIKey), nil

	case SwqosTypeZeroSlot:
		return NewZeroSlotClient(rpcURL, config.CustomURL, config.APIKey), nil

	case SwqosTypeTemporal:
		return NewTemporalClient(rpcURL, config.CustomURL, config.APIKey), nil

	case SwqosTypeFlashBlock:
		return NewFlashBlockClient(rpcURL, config.CustomURL, config.APIKey), nil

	case SwqosTypeBlockRazor:
		endpoint := "api.blockrazor.com"
		if config.CustomURL != "" {
			endpoint = config.CustomURL
		}
		return NewBlockRazorClient(rpcURL, endpoint, config.APIKey, config.MEVProtection), nil

	case SwqosTypeAstralane:
		endpoint := "api.astralane.com"
		if config.CustomURL != "" {
			endpoint = config.CustomURL
		} else if config.MEVProtection {
			endpoint = "api-mev.astralane.com:9000"
		}
		return NewAstralaneClient(rpcURL, endpoint, config.APIKey), nil

	case SwqosTypeHelius:
		return NewHeliusClient(rpcURL, config.CustomURL, &config.APIKey, false), nil

	case SwqosTypeDefault:
		return NewDefaultClient(rpcURL), nil

	default:
		return nil, fmt.Errorf("unsupported SWQOS type: %v", config.Type)
	}
}
