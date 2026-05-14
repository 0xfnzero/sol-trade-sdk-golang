package common

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// ===== SubscriptionHandle =====

// SubscriptionID represents a unique subscription identifier
type SubscriptionID uint64

// SubscriptionHandle represents an active subscription
type SubscriptionHandle struct {
	id          SubscriptionID
	method      string
	params      interface{}
	callback    func([]byte)
	errCallback func(error)
	ctx         context.Context
	cancel      context.CancelFunc
	active      atomic.Bool
	subscribed  atomic.Bool
	mu          sync.RWMutex
}

// NewSubscriptionHandle creates a new subscription handle
func NewSubscriptionHandle(
	id SubscriptionID,
	method string,
	params interface{},
	callback func([]byte),
	errCallback func(error),
) *SubscriptionHandle {
	ctx, cancel := context.WithCancel(context.Background())
	return &SubscriptionHandle{
		id:          id,
		method:      method,
		params:      params,
		callback:    callback,
		errCallback: errCallback,
		ctx:         ctx,
		cancel:      cancel,
	}
}

// ID returns the subscription ID
func (h *SubscriptionHandle) ID() SubscriptionID {
	return h.id
}

// Method returns the subscription method
func (h *SubscriptionHandle) Method() string {
	return h.method
}

// Params returns the subscription parameters
func (h *SubscriptionHandle) Params() interface{} {
	return h.params
}

// IsActive returns true if the subscription is active
func (h *SubscriptionHandle) IsActive() bool {
	return h.active.Load()
}

// IsSubscribed returns true if successfully subscribed
func (h *SubscriptionHandle) IsSubscribed() bool {
	return h.subscribed.Load()
}

// Context returns the subscription context
func (h *SubscriptionHandle) Context() context.Context {
	return h.ctx
}

// Unsubscribe cancels the subscription
func (h *SubscriptionHandle) Unsubscribe() {
	if h.active.CompareAndSwap(true, false) {
		h.cancel()
	}
}

// setSubscribed marks the subscription as successfully subscribed
func (h *SubscriptionHandle) setSubscribed() {
	h.subscribed.Store(true)
	h.active.Store(true)
}

// notify calls the callback with data
func (h *SubscriptionHandle) notify(data []byte) {
	if h.IsActive() && h.callback != nil {
		h.callback(data)
	}
}

// notifyError calls the error callback
func (h *SubscriptionHandle) notifyError(err error) {
	if h.IsActive() && h.errCallback != nil {
		h.errCallback(err)
	}
}

// ===== WebSocket Message Types =====

// WSRequest represents a WebSocket request
type WSRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      uint64      `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

// WSResponse represents a WebSocket response
type WSResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      uint64          `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *WSError        `json:"error,omitempty"`
}

// WSNotification represents a subscription notification
type WSNotification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// WSError represents a WebSocket error
type WSError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *WSError) Error() string {
	return fmt.Sprintf("WebSocket error %d: %s", e.Code, e.Message)
}

// ===== SubscriptionManager =====

// SubscriptionManager manages WebSocket subscriptions
type SubscriptionManager struct {
	endpoint        string
	conn            *websocket.Conn
	subscriptions   sync.Map // map[SubscriptionID]*SubscriptionHandle
	subIDToHandleID sync.Map // map[uint64]SubscriptionID (server sub ID -> local handle ID)
	nextID          atomic.Uint64
	requestID       atomic.Uint64

	// Connection management
	connected    atomic.Bool
	connecting   atomic.Bool
	reconnecting atomic.Bool
	stopCh       chan struct{}
	mu           sync.RWMutex

	// Configuration
	dialer         *websocket.Dialer
	reconnectDelay time.Duration
	pingInterval   time.Duration
	writeTimeout   time.Duration
	headers        http.Header

	// Callbacks
	onConnect    func()
	onDisconnect func(error)
	onError      func(error)
}

// SubscriptionManagerOption is a functional option for the manager
type SubscriptionManagerOption func(*SubscriptionManager)

// WithReconnectDelay sets the reconnect delay
func WithReconnectDelay(delay time.Duration) SubscriptionManagerOption {
	return func(m *SubscriptionManager) {
		m.reconnectDelay = delay
	}
}

// WithPingInterval sets the ping interval
func WithPingInterval(interval time.Duration) SubscriptionManagerOption {
	return func(m *SubscriptionManager) {
		m.pingInterval = interval
	}
}

// WithWriteTimeout sets the write timeout
func WithWriteTimeout(timeout time.Duration) SubscriptionManagerOption {
	return func(m *SubscriptionManager) {
		m.writeTimeout = timeout
	}
}

// WithHeaders sets custom headers
func WithHeaders(headers http.Header) SubscriptionManagerOption {
	return func(m *SubscriptionManager) {
		m.headers = headers
	}
}

// WithOnConnect sets the connect callback
func WithOnConnect(fn func()) SubscriptionManagerOption {
	return func(m *SubscriptionManager) {
		m.onConnect = fn
	}
}

// WithOnDisconnect sets the disconnect callback
func WithOnDisconnect(fn func(error)) SubscriptionManagerOption {
	return func(m *SubscriptionManager) {
		m.onDisconnect = fn
	}
}

// WithOnError sets the error callback
func WithOnError(fn func(error)) SubscriptionManagerOption {
	return func(m *SubscriptionManager) {
		m.onError = fn
	}
}

// NewSubscriptionManager creates a new subscription manager
func NewSubscriptionManager(endpoint string, opts ...SubscriptionManagerOption) *SubscriptionManager {
	m := &SubscriptionManager{
		endpoint:       endpoint,
		stopCh:         make(chan struct{}),
		reconnectDelay: 5 * time.Second,
		pingInterval:   30 * time.Second,
		writeTimeout:   10 * time.Second,
		dialer: &websocket.Dialer{
			HandshakeTimeout: 10 * time.Second,
			ReadBufferSize:   1024 * 1024,
			WriteBufferSize:  1024 * 1024,
		},
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// Connect establishes the WebSocket connection
func (m *SubscriptionManager) Connect(ctx context.Context) error {
	if m.connected.Load() {
		return nil
	}

	if !m.connecting.CompareAndSwap(false, true) {
		return errors.New("already connecting")
	}
	defer m.connecting.Store(false)

	conn, _, err := m.dialer.DialContext(ctx, m.endpoint, m.headers)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	m.mu.Lock()
	m.conn = conn
	m.mu.Unlock()

	m.connected.Store(true)

	// Start goroutines
	go m.readLoop()
	go m.pingLoop()

	if m.onConnect != nil {
		m.onConnect()
	}

	// Resubscribe to existing subscriptions
	m.resubscribeAll()

	return nil
}

// Disconnect closes the WebSocket connection
func (m *SubscriptionManager) Disconnect() error {
	close(m.stopCh)

	m.connected.Store(false)

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.conn != nil {
		// Unsubscribe all
		m.subscriptions.Range(func(key, value interface{}) bool {
			handle := value.(*SubscriptionHandle)
			handle.Unsubscribe()
			return true
		})

		err := m.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			m.conn.Close()
			return err
		}
		return m.conn.Close()
	}

	return nil
}

// IsConnected returns true if connected
func (m *SubscriptionManager) IsConnected() bool {
	return m.connected.Load()
}

// Subscribe creates a new subscription
func (m *SubscriptionManager) Subscribe(
	method string,
	params interface{},
	callback func([]byte),
	errCallback func(error),
) (*SubscriptionHandle, error) {
	if !m.connected.Load() {
		return nil, errors.New("not connected")
	}

	id := SubscriptionID(m.nextID.Add(1))
	handle := NewSubscriptionHandle(id, method, params, callback, errCallback)

	// Store subscription
	m.subscriptions.Store(id, handle)

	// Send subscribe request
	if err := m.sendSubscribe(handle); err != nil {
		m.subscriptions.Delete(id)
		return nil, err
	}

	return handle, nil
}

// Unsubscribe removes a subscription
func (m *SubscriptionManager) Unsubscribe(handle *SubscriptionHandle) error {
	if handle == nil {
		return nil
	}

	handle.Unsubscribe()

	// Send unsubscribe if subscribed
	if handle.IsSubscribed() {
		if err := m.sendUnsubscribe(handle); err != nil {
			return err
		}
	}

	m.subscriptions.Delete(handle.ID())
	return nil
}

// sendSubscribe sends a subscribe request
func (m *SubscriptionManager) sendSubscribe(handle *SubscriptionHandle) error {
	reqID := m.requestID.Add(1)

	req := WSRequest{
		JSONRPC: "2.0",
		ID:      reqID,
		Method:  handle.Method(),
		Params:  handle.Params(),
	}

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	return m.write(data)
}

// sendUnsubscribe sends an unsubscribe request
func (m *SubscriptionManager) sendUnsubscribe(handle *SubscriptionHandle) error {
	// Different methods have different unsubscribe formats
	var method string
	switch handle.Method() {
	case "accountSubscribe":
		method = "accountUnsubscribe"
	case "logsSubscribe":
		method = "logsUnsubscribe"
	case "programSubscribe":
		method = "programUnsubscribe"
	case "signatureSubscribe":
		method = "signatureUnsubscribe"
	case "slotSubscribe":
		method = "slotUnsubscribe"
	case "blockSubscribe":
		method = "blockUnsubscribe"
	default:
		return nil // Unknown method, skip unsubscribe
	}

	reqID := m.requestID.Add(1)
	req := WSRequest{
		JSONRPC: "2.0",
		ID:      reqID,
		Method:  method,
		Params:  []interface{}{handle.ID()},
	}

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal unsubscribe: %w", err)
	}

	return m.write(data)
}

// resubscribeAll resubscribes to all active subscriptions after reconnect
func (m *SubscriptionManager) resubscribeAll() {
	m.subscriptions.Range(func(key, value interface{}) bool {
		handle := value.(*SubscriptionHandle)
		if handle.IsActive() {
			m.sendSubscribe(handle)
		}
		return true
	})
}

// write sends data over the WebSocket
func (m *SubscriptionManager) write(data []byte) error {
	m.mu.RLock()
	conn := m.conn
	m.mu.RUnlock()

	if conn == nil {
		return errors.New("not connected")
	}

	conn.SetWriteDeadline(time.Now().Add(m.writeTimeout))
	return conn.WriteMessage(websocket.TextMessage, data)
}

// readLoop reads messages from the WebSocket
func (m *SubscriptionManager) readLoop() {
	defer func() {
		if r := recover(); r != nil {
			m.handleError(fmt.Errorf("panic in read loop: %v", r))
		}
	}()

	for {
		select {
		case <-m.stopCh:
			return
		default:
		}

		m.mu.RLock()
		conn := m.conn
		m.mu.RUnlock()

		if conn == nil {
			return
		}

		_, data, err := conn.ReadMessage()
		if err != nil {
			m.handleDisconnect(err)
			return
		}

		m.handleMessage(data)
	}
}

// pingLoop sends periodic ping messages
func (m *SubscriptionManager) pingLoop() {
	ticker := time.NewTicker(m.pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			if !m.connected.Load() {
				return
			}

			m.mu.RLock()
			conn := m.conn
			m.mu.RUnlock()

			if conn != nil {
				conn.SetWriteDeadline(time.Now().Add(m.writeTimeout))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					m.handleDisconnect(err)
					return
				}
			}
		}
	}
}

// handleMessage processes incoming messages
func (m *SubscriptionManager) handleMessage(data []byte) {
	// Try to parse as notification first
	var notification WSNotification
	if err := json.Unmarshal(data, &notification); err == nil && notification.Method == "subscription" {
		m.handleNotification(data)
		return
	}

	// Try to parse as response
	var response WSResponse
	if err := json.Unmarshal(data, &response); err == nil {
		m.handleResponse(&response)
		return
	}

	// Unknown message format
	m.handleError(fmt.Errorf("unknown message format: %s", string(data)))
}

// handleNotification processes subscription notifications
func (m *SubscriptionManager) handleNotification(data []byte) {
	var notif struct {
		JSONRPC string `json:"jsonrpc"`
		Method  string `json:"method"`
		Params  struct {
			Result       json.RawMessage `json:"result"`
			Subscription uint64          `json:"subscription"`
		} `json:"params"`
	}

	if err := json.Unmarshal(data, &notif); err != nil {
		m.handleError(fmt.Errorf("failed to unmarshal notification: %w", err))
		return
	}

	// Find the handle by subscription ID
	handleID, ok := m.subIDToHandleID.Load(notif.Params.Subscription)
	if !ok {
		return // Unknown subscription
	}

	handle, ok := m.subscriptions.Load(handleID)
	if !ok {
		return // Handle not found
	}

	handle.(*SubscriptionHandle).notify(notif.Params.Result)
}

// handleResponse processes RPC responses
func (m *SubscriptionManager) handleResponse(resp *WSResponse) {
	if resp.Error != nil {
		m.handleError(resp.Error)
		return
	}

	// For subscription responses, the result is the subscription ID
	var subID uint64
	if err := json.Unmarshal(resp.Result, &subID); err == nil {
		// Map server subscription ID to local handle ID
		// In a real implementation, we'd track pending subscriptions
		_ = subID
	}
}

// handleDisconnect handles disconnection
func (m *SubscriptionManager) handleDisconnect(err error) {
	m.connected.Store(false)

	if m.onDisconnect != nil {
		m.onDisconnect(err)
	}

	// Attempt reconnection if not explicitly stopped
	select {
	case <-m.stopCh:
		return
	default:
		go m.reconnect()
	}
}

// reconnect attempts to reconnect
func (m *SubscriptionManager) reconnect() {
	if !m.reconnecting.CompareAndSwap(false, true) {
		return
	}
	defer m.reconnecting.Store(false)

	for {
		select {
		case <-m.stopCh:
			return
		case <-time.After(m.reconnectDelay):
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err := m.Connect(ctx)
		cancel()

		if err == nil {
			return // Successfully reconnected
		}

		// Exponential backoff
		m.reconnectDelay *= 2
		if m.reconnectDelay > 60*time.Second {
			m.reconnectDelay = 60 * time.Second
		}
	}
}

// handleError handles errors
func (m *SubscriptionManager) handleError(err error) {
	if m.onError != nil {
		m.onError(err)
	}
}

// ===== Convenience Methods =====

// SubscribeAccount subscribes to account changes
func (m *SubscriptionManager) SubscribeAccount(
	pubkey string,
	encoding string,
	callback func([]byte),
	errCallback func(error),
) (*SubscriptionHandle, error) {
	params := map[string]interface{}{
		"encoding":   encoding,
		"commitment": "confirmed",
	}
	return m.Subscribe("accountSubscribe", []interface{}{pubkey, params}, callback, errCallback)
}

// SubscribeLogs subscribes to transaction logs
func (m *SubscriptionManager) SubscribeLogs(
	mentions []string,
	callback func([]byte),
	errCallback func(error),
) (*SubscriptionHandle, error) {
	params := map[string]interface{}{
		"mentions": mentions,
	}
	return m.Subscribe("logsSubscribe", []interface{}{params}, callback, errCallback)
}

// SubscribeProgram subscribes to program account changes
func (m *SubscriptionManager) SubscribeProgram(
	programID string,
	encoding string,
	callback func([]byte),
	errCallback func(error),
) (*SubscriptionHandle, error) {
	params := map[string]interface{}{
		"encoding":   encoding,
		"commitment": "confirmed",
	}
	return m.Subscribe("programSubscribe", []interface{}{programID, params}, callback, errCallback)
}

// SubscribeSignature subscribes to signature status
func (m *SubscriptionManager) SubscribeSignature(
	signature string,
	callback func([]byte),
	errCallback func(error),
) (*SubscriptionHandle, error) {
	params := map[string]interface{}{
		"commitment": "confirmed",
	}
	return m.Subscribe("signatureSubscribe", []interface{}{signature, params}, callback, errCallback)
}

// SubscribeSlot subscribes to slot updates
func (m *SubscriptionManager) SubscribeSlot(
	callback func([]byte),
	errCallback func(error),
) (*SubscriptionHandle, error) {
	return m.Subscribe("slotSubscribe", []interface{}{}, callback, errCallback)
}

// ===== Errors =====

var (
	ErrNotConnected       = errors.New("not connected to WebSocket")
	ErrAlreadyConnecting  = errors.New("already connecting")
	ErrInvalidParams      = errors.New("invalid subscription parameters")
	ErrSubscriptionFailed = errors.New("subscription failed")
)
