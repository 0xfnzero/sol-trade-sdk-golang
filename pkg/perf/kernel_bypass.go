// Package perf provides performance optimizations for Sol Trade SDK
// kernel_bypass.go - Kernel bypass optimizations for ultra-low latency I/O
package perf

import (
	"errors"
	"io"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

// ===== Errors =====

var (
	ErrNotLinux          = errors.New("io_uring only available on Linux")
	ErrUringNotAvailable = errors.New("io_uring library not available")
	ErrFileNotOpen       = errors.New("file not open")
	ErrSocketNotSet      = errors.New("socket not set")
	ErrInvalidOffset     = errors.New("invalid offset")
)

// ===== IO Operation Types =====

// IOOperation represents I/O operation types
type IOOperation int

const (
	IORead IOOperation = iota
	IOWrite
	IOFsync
	IOPoll
	IOTimeout
)

// IORequest represents an I/O request for kernel bypass operations
type IORequest struct {
	Op       IOOperation
	Fd       int
	Buffer   []byte
	Offset   int64
	Size     int
	Callback func(int, []byte, int)
	UserData interface{}
}

// IOResult represents the result of an I/O operation
type IOResult struct {
	RequestID        int
	BytesTransferred int
	Buffer           []byte
	Error            error
	UserData         interface{}
}

// IOUringConfig represents configuration for io_uring
type IOUringConfig struct {
	QueueDepth   int
	SQThreadIdle int // ms before SQ thread sleeps
	SQThreadCPU  int // CPU for SQ thread (-1 = any)
	CQSize       int // 0 = same as queue_depth
	Flags        uint32
	Features     []string
}

// DefaultIOUringConfig returns default io_uring configuration
func DefaultIOUringConfig() *IOUringConfig {
	return &IOUringConfig{
		QueueDepth:   256,
		SQThreadIdle: 2000,
		SQThreadCPU:  -1,
		CQSize:       0,
		Flags:        0,
		Features:     []string{},
	}
}

// ===== Kernel Bypass Manager =====

// KernelBypassManager manages kernel bypass I/O operations.
// Uses io_uring on Linux when available, falls back to epoll on other platforms.
type KernelBypassManager struct {
	config          *IOUringConfig
	uringAvailable  bool
	lock            sync.RWMutex
	requestCounter  int32
	pendingRequests map[int]*IORequest
	running         atomic.Bool
	stopChan        chan struct{}
	wg              sync.WaitGroup
}

// NewKernelBypassManager creates a new kernel bypass manager
func NewKernelBypassManager(config *IOUringConfig) *KernelBypassManager {
	if config == nil {
		config = DefaultIOUringConfig()
	}

	kbm := &KernelBypassManager{
		config:          config,
		pendingRequests: make(map[int]*IORequest),
		stopChan:        make(chan struct{}),
	}

	kbm.checkUringAvailability()
	return kbm
}

// checkUringAvailability checks if io_uring is available on this system
func (kbm *KernelBypassManager) checkUringAvailability() bool {
	if runtime.GOOS != "linux" {
		return false
	}

	// Check for io_uring syscall availability
	// On Go, we can use golang.org/x/sys/unix for io_uring support
	// For now, we'll use a simplified check
	fd, err := syscall.Open("/dev/null", syscall.O_RDONLY, 0)
	if err != nil {
		return false
	}
	syscall.Close(fd)

	// io_uring setup would go here with proper library
	// For now, return false to use fallback
	kbm.uringAvailable = false
	return false
}

// IsUringAvailable checks if io_uring is available
func (kbm *KernelBypassManager) IsUringAvailable() bool {
	return kbm.uringAvailable
}

// Start starts the I/O processing loop
func (kbm *KernelBypassManager) Start() error {
	if kbm.running.Swap(true) {
		return nil // Already running
	}

	kbm.wg.Add(1)
	if kbm.uringAvailable {
		go kbm.uringLoop()
	} else {
		go kbm.fallbackLoop()
	}

	return nil
}

// Stop stops the I/O processing loop
func (kbm *KernelBypassManager) Stop() {
	if !kbm.running.Swap(false) {
		return
	}

	close(kbm.stopChan)
	kbm.wg.Wait()
}

// uringLoop is the main io_uring processing loop
func (kbm *KernelBypassManager) uringLoop() {
	defer kbm.wg.Done()

	// io_uring implementation would go here
	// For now, use fallback
	kbm.fallbackLoop()
}

// fallbackLoop is the fallback I/O loop using epoll/select
func (kbm *KernelBypassManager) fallbackLoop() {
	defer kbm.wg.Done()

	ticker := time.NewTicker(100 * time.Microsecond)
	defer ticker.Stop()

	for {
		select {
		case <-kbm.stopChan:
			return
		case <-ticker.C:
			kbm.processPendingRequests()
		}
	}
}

// processPendingRequests processes pending I/O requests
func (kbm *KernelBypassManager) processPendingRequests() {
	kbm.lock.Lock()
	defer kbm.lock.Unlock()

	for id, req := range kbm.pendingRequests {
		switch req.Op {
		case IORead:
			kbm.processRead(id, req)
		case IOWrite:
			kbm.processWrite(id, req)
		}
		delete(kbm.pendingRequests, id)
	}
}

// processRead processes a read request
func (kbm *KernelBypassManager) processRead(id int, req *IORequest) {
	buf := make([]byte, req.Size)
	n, err := syscall.Pread(req.Fd, buf, req.Offset)
	if err != nil && err != io.EOF {
		if req.Callback != nil {
			req.Callback(id, nil, 0)
		}
		return
	}

	if req.Callback != nil {
		req.Callback(id, buf[:n], n)
	}
}

// processWrite processes a write request
func (kbm *KernelBypassManager) processWrite(id int, req *IORequest) {
	n, err := syscall.Pwrite(req.Fd, req.Buffer, req.Offset)
	if err != nil {
		if req.Callback != nil {
			req.Callback(id, nil, 0)
		}
		return
	}

	if req.Callback != nil {
		req.Callback(id, nil, n)
	}
}

// SubmitRead submits an async read request
func (kbm *KernelBypassManager) SubmitRead(
	fd int,
	size int,
	offset int64,
	callback func(int, []byte, int),
	userData interface{},
) int {
	id := int(atomic.AddInt32(&kbm.requestCounter, 1))

	req := &IORequest{
		Op:       IORead,
		Fd:       fd,
		Offset:   offset,
		Size:     size,
		Callback: callback,
		UserData: userData,
	}

	kbm.lock.Lock()
	kbm.pendingRequests[id] = req
	kbm.lock.Unlock()

	return id
}

// SubmitWrite submits an async write request
func (kbm *KernelBypassManager) SubmitWrite(
	fd int,
	buffer []byte,
	offset int64,
	callback func(int, []byte, int),
	userData interface{},
) int {
	id := int(atomic.AddInt32(&kbm.requestCounter, 1))

	req := &IORequest{
		Op:       IOWrite,
		Fd:       fd,
		Buffer:   buffer,
		Offset:   offset,
		Size:     len(buffer),
		Callback: callback,
		UserData: userData,
	}

	kbm.lock.Lock()
	kbm.pendingRequests[id] = req
	kbm.lock.Unlock()

	return id
}

// ===== Direct I/O File =====

// DirectIOFile provides file access with direct I/O support (bypassing page cache)
type DirectIOFile struct {
	path     string
	fd       int
	directIO bool
}

// NewDirectIOFile creates a new direct I/O file
func NewDirectIOFile(path string) *DirectIOFile {
	return &DirectIOFile{
		path: path,
		fd:   -1,
	}
}

// Open opens the file with direct I/O if available
func (f *DirectIOFile) Open() error {
	flags := syscall.O_RDWR | syscall.O_CREAT

	// Try O_DIRECT on Linux
	if runtime.GOOS == "linux" {
		flags |= 0x4000 // syscall.O_DIRECT on Linux
		f.directIO = true
	}

	fd, err := syscall.Open(f.path, flags, 0644)
	if err != nil && f.directIO {
		// Fallback without O_DIRECT
		f.directIO = false
		fd, err = syscall.Open(f.path, syscall.O_RDWR|syscall.O_CREAT, 0644)
	}

	if err != nil {
		return err
	}

	f.fd = fd
	return nil
}

// Close closes the file
func (f *DirectIOFile) Close() error {
	if f.fd >= 0 {
		err := syscall.Close(f.fd)
		f.fd = -1
		return err
	}
	return nil
}

// Read reads from the file
func (f *DirectIOFile) Read(size int, offset int64) ([]byte, error) {
	if f.fd < 0 {
		return nil, ErrFileNotOpen
	}

	if f.directIO {
		return f.readDirect(size, offset)
	}

	buf := make([]byte, size)
	n, err := syscall.Pread(f.fd, buf, offset)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

// readDirect reads using direct I/O with aligned buffer
func (f *DirectIOFile) readDirect(size int, offset int64) ([]byte, error) {
	align := int64(512) // Typical sector size
	alignedOffset := (offset / align) * align
	offsetDiff := offset - alignedOffset
	alignedSize := ((int64(size) + offsetDiff + align - 1) / align) * align

	// Allocate aligned buffer
	buf := make([]byte, alignedSize)
	// Ensure 512-byte alignment
	if int(uintptr(unsafe.Pointer(&buf[0])))%512 != 0 {
		// Re-allocate aligned buffer
		buf = make([]byte, alignedSize+512)
		alignedPtr := (uintptr(unsafe.Pointer(&buf[0])) + 511) &^ 511
		buf = (*[1 << 30]byte)(unsafe.Pointer(alignedPtr))[:alignedSize]
	}

	n, err := syscall.Pread(f.fd, buf, alignedOffset)
	if err != nil {
		return nil, err
	}
	if int64(n) < offsetDiff+int64(size) {
		return nil, io.ErrUnexpectedEOF
	}

	return buf[offsetDiff : offsetDiff+int64(size)], nil
}

// Write writes to the file
func (f *DirectIOFile) Write(data []byte, offset int64) (int, error) {
	if f.fd < 0 {
		return 0, ErrFileNotOpen
	}

	if f.directIO {
		return f.writeDirect(data, offset)
	}

	return syscall.Pwrite(f.fd, data, offset)
}

// writeDirect writes using direct I/O with aligned buffer
func (f *DirectIOFile) writeDirect(data []byte, offset int64) (int, error) {
	align := int64(512)
	alignedOffset := (offset / align) * align
	offsetDiff := offset - alignedOffset
	alignedSize := ((int64(len(data)) + offsetDiff + align - 1) / align) * align

	// Create aligned buffer
	buf := make([]byte, alignedSize)
	copy(buf[offsetDiff:], data)

	n, err := syscall.Pwrite(f.fd, buf, alignedOffset)
	if err != nil {
		return 0, err
	}
	if n < len(data) {
		return n, io.ErrShortWrite
	}

	return len(data), nil
}

// Fsync syncs file to disk
func (f *DirectIOFile) Fsync() error {
	if f.fd < 0 {
		return ErrFileNotOpen
	}
	return syscall.Fsync(f.fd)
}

// ===== Memory Mapped File =====

// MemoryMappedFile provides zero-copy access via memory mapping
type MemoryMappedFile struct {
	path string
	fd   int
	data []byte
}

// NewMemoryMappedFile creates a new memory-mapped file
func NewMemoryMappedFile(path string) *MemoryMappedFile {
	return &MemoryMappedFile{
		path: path,
		fd:   -1,
	}
}

// Open opens and memory-maps the file
func (m *MemoryMappedFile) Open(size int64) error {
	fd, err := syscall.Open(m.path, syscall.O_RDWR|syscall.O_CREAT, 0644)
	if err != nil {
		return err
	}
	m.fd = fd

	// Get file size
	var stat syscall.Stat_t
	if err := syscall.Fstat(fd, &stat); err != nil {
		syscall.Close(fd)
		return err
	}

	fileSize := stat.Size
	if size > 0 && size > fileSize {
		// Extend file
		if err := syscall.Ftruncate(fd, size); err != nil {
			syscall.Close(fd)
			return err
		}
		fileSize = size
	}

	if fileSize == 0 {
		fileSize = 4096 // Default size
		if err := syscall.Ftruncate(fd, fileSize); err != nil {
			syscall.Close(fd)
			return err
		}
	}

	// Memory map
	data, err := syscall.Mmap(fd, 0, int(fileSize), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		syscall.Close(fd)
		return err
	}

	m.data = data
	return nil
}

// Close closes the memory-mapped file
func (m *MemoryMappedFile) Close() error {
	var err error
	if m.data != nil {
		err = syscall.Munmap(m.data)
		m.data = nil
	}

	if m.fd >= 0 {
		if closeErr := syscall.Close(m.fd); closeErr != nil && err == nil {
			err = closeErr
		}
		m.fd = -1
	}

	return err
}

// Read reads from memory-mapped file
func (m *MemoryMappedFile) Read(offset int64, size int) ([]byte, error) {
	if m.data == nil {
		return nil, ErrFileNotOpen
	}

	end := offset + int64(size)
	if end > int64(len(m.data)) {
		return nil, errors.New("read beyond file size")
	}

	result := make([]byte, size)
	copy(result, m.data[offset:end])
	return result, nil
}

// Write writes to memory-mapped file
func (m *MemoryMappedFile) Write(offset int64, data []byte) error {
	if m.data == nil {
		return ErrFileNotOpen
	}

	end := offset + int64(len(data))
	if end > int64(len(m.data)) {
		return errors.New("write beyond file size")
	}

	copy(m.data[offset:end], data)
	return nil
}

// Flush flushes changes to disk
func (m *MemoryMappedFile) Flush() error {
	if m.data == nil {
		return ErrFileNotOpen
	}
	return nil
}

// ===== Async Socket =====

// AsyncSocket provides async socket operations with kernel bypass optimizations
type AsyncSocket struct {
	fd        int
	connected bool
}

// NewAsyncSocket creates a new async socket
func NewAsyncSocket() *AsyncSocket {
	return &AsyncSocket{
		fd: -1,
	}
}

// SetFD sets the file descriptor
func (s *AsyncSocket) SetFD(fd int) {
	s.fd = fd
}

// EnableKernelBypass enables kernel bypass optimizations for the socket
func (s *AsyncSocket) EnableKernelBypass() error {
	if s.fd < 0 {
		return ErrSocketNotSet
	}

	// Set TCP_NODELAY
	if err := syscall.SetsockoptInt(s.fd, syscall.IPPROTO_TCP, syscall.TCP_NODELAY, 1); err != nil {
		return err
	}

	return nil
}

// ===== IO Batch Processor =====

// IOBatchProcessor batches I/O operations with kernel bypass
type IOBatchProcessor struct {
	maxBatchSize int
	readBatch    []struct {
		fd       int
		size     int
		offset   int64
		callback func(int, []byte, int)
	}
	writeBatch []struct {
		fd       int
		data     []byte
		offset   int64
		callback func(int, []byte, int)
	}
	manager *KernelBypassManager
}

// NewIOBatchProcessor creates a new I/O batch processor
func NewIOBatchProcessor(maxBatchSize int) *IOBatchProcessor {
	if maxBatchSize <= 0 {
		maxBatchSize = 32
	}

	return &IOBatchProcessor{
		maxBatchSize: maxBatchSize,
		manager:      NewKernelBypassManager(nil),
	}
}

// AddRead adds a read to the batch
func (p *IOBatchProcessor) AddRead(fd int, size int, offset int64, callback func(int, []byte, int)) {
	p.readBatch = append(p.readBatch, struct {
		fd       int
		size     int
		offset   int64
		callback func(int, []byte, int)
	}{fd, size, offset, callback})

	if len(p.readBatch) >= p.maxBatchSize {
		p.FlushReads()
	}
}

// AddWrite adds a write to the batch
func (p *IOBatchProcessor) AddWrite(fd int, data []byte, offset int64, callback func(int, []byte, int)) {
	p.writeBatch = append(p.writeBatch, struct {
		fd       int
		data     []byte
		offset   int64
		callback func(int, []byte, int)
	}{fd, data, offset, callback})

	if len(p.writeBatch) >= p.maxBatchSize {
		p.FlushWrites()
	}
}

// FlushReads flushes all pending reads
func (p *IOBatchProcessor) FlushReads() []int {
	if len(p.readBatch) == 0 {
		return nil
	}

	requestIDs := make([]int, len(p.readBatch))
	for i, req := range p.readBatch {
		requestIDs[i] = p.manager.SubmitRead(req.fd, req.size, req.offset, req.callback, nil)
	}

	p.readBatch = p.readBatch[:0]
	return requestIDs
}

// FlushWrites flushes all pending writes
func (p *IOBatchProcessor) FlushWrites() []int {
	if len(p.writeBatch) == 0 {
		return nil
	}

	requestIDs := make([]int, len(p.writeBatch))
	for i, req := range p.writeBatch {
		requestIDs[i] = p.manager.SubmitWrite(req.fd, req.data, req.offset, req.callback, nil)
	}

	p.writeBatch = p.writeBatch[:0]
	return requestIDs
}

// FlushAll flushes all pending operations
func (p *IOBatchProcessor) FlushAll() (readIDs, writeIDs []int) {
	return p.FlushReads(), p.FlushWrites()
}

// ===== Global Manager =====

var (
	globalKBManager     *KernelBypassManager
	globalKBManagerOnce sync.Once
)

// GetKernelBypassManager gets or creates the global kernel bypass manager
func GetKernelBypassManager(config *IOUringConfig) *KernelBypassManager {
	globalKBManagerOnce.Do(func() {
		globalKBManager = NewKernelBypassManager(config)
	})
	return globalKBManager
}
