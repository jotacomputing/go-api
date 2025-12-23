package queue

import (
	"fmt"
	"os"
	"sync/atomic"
	"unsafe"
	"github.com/edsrzf/mmap-go"
	"jotacomputing/go-api/structs"
	
	"log"
)

type QueryQueueHeader struct {
	ProducerHead uint64   // Offset 0 4 byte interger 
	_pad1        [56]byte // Padding to cache line
	ConsumerTail uint64   // Offset 64
	_pad2        [56]byte // Padding
	Magic        uint32   // Offset 128
	Capacity     uint32   // Offset 132
}

const QuerySize = unsafe.Sizeof(structs.Query{})
const QueryHeaderSize = unsafe.Sizeof(QueryQueueHeader{})
const TotalQuerySize = QueryHeaderSize + (QueueCapacity * QuerySize)

type QueryQueue struct {
	file   *os.File
	mmap   mmap.MMap   // this is the array of bytes wich we will use to read and write 
	header *QueryQueueHeader
	orders []structs.Query
}


func InitQueryQueue(filePath string) {
	fmt.Println("[INIT] Initializing shared memory queue...")

	q, err := CreateQueryQueue(filePath)
	if err != nil {
		log.Fatalf("Failed to create queue: %v", err)
	}
	defer q.Close()

	fmt.Printf("[INIT] Queue initialized successfully\n")
	fmt.Printf("[INIT] Capacity: %d orders\n", q.Capacity())
	fmt.Printf("[INIT] Queue depth: %d\n", q.Depth())
	fmt.Printf("[INIT] File: %s (size: ~3.2 MB)\n", filePath)

	// Create status queue too
	fmt.Println("\n[INIT] Initializing status feedback queue...")
	statusQ, err := CreateQueue(filePath + "_status")
	if err != nil {
		log.Fatalf("Failed to create status queue: %v", err)
	}
	defer statusQ.Close()

	fmt.Printf("[INIT] Status queue initialized successfully\n")
	fmt.Printf("[INIT] File: %s_status (size: ~3.2 MB)\n", filePath)
}

func CreateQueryQueue(filePath string) (*QueryQueue, error) {
	_ = os.Remove(filePath)

	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	// set the size of the file
	if err := file.Truncate(int64(TotalQuerySize)); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to truncate file: %w", err)
	}

	// sync to disk before mmap
	if err := file.Sync(); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to sync file: %w", err)
	}
	// m is just a byte array that is mapped to the real file on the Ram 
	m, err := mmap.Map(file, mmap.RDWR, 0)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to mmap: %w", err)
	}

	// try to lock in RAM
	if err := m.Lock(); err != nil {
		// proceed without locking;
		// caller may tune ulimit -l / CAP_IPC_LOCK
	}

	// initialize header
	header := (*QueryQueueHeader)(unsafe.Pointer(&m[0]))
	atomic.StoreUint64(&header.ProducerHead, 0)
	atomic.StoreUint64(&header.ConsumerTail, 0)
	atomic.StoreUint32(&header.Magic, QueueMagic)
	atomic.StoreUint32(&header.Capacity, QueueCapacity)

	// flush to disk
	if err := m.Flush(); err != nil {
		m.Unlock()
		m.Unmap()
		file.Close()
		return nil, fmt.Errorf("failed to flush mmap: %w", err)
	}

	ordersData := m[int(QueryHeaderSize):int(TotalQuerySize)]
	if len(ordersData) == 0 {
		m.Unlock()
		m.Unmap()
		file.Close()
		return nil, fmt.Errorf("orders region empty")
	}
	orders := unsafe.Slice((*structs.Query)(unsafe.Pointer(&ordersData[0])), QueueCapacity)

	return &QueryQueue{
		file:   file,
		mmap:   m,
		header: header,
		orders: orders,
	}, nil
}

func OpenQueryQueue(filePath string) (*QueryQueue, error) {
	file, err := os.OpenFile(filePath, os.O_RDWR, 0o666)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	// verify file size
	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}
	if stat.Size() != int64(TotalQuerySize) {
		file.Close()
		return nil, fmt.Errorf("invalid file size: got %d, expected %d", stat.Size(), int64(TotalQuerySize))
	}

	m, err := mmap.Map(file, mmap.RDWR, 0)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to mmap: %w", err)
	}

	if err := m.Lock(); err != nil {
		// non-fatal; continue without lock
	}

	// validate header
	header := (*QueryQueueHeader)(unsafe.Pointer(&m[0]))
	if atomic.LoadUint32(&header.Magic) != QueueMagic {
		m.Unlock()
		m.Unmap()
		file.Close()
		return nil, fmt.Errorf("invalid queue magic number")
	}
	if atomic.LoadUint32(&header.Capacity) != QueueCapacity {
		m.Unlock()
		m.Unmap()
		file.Close()
		return nil, fmt.Errorf("capacity mismatch: file=%d code=%d", header.Capacity, QueueCapacity)
	}

	ordersData := m[int(QueryHeaderSize):int(TotalQuerySize)]
	if len(ordersData) == 0 {
		m.Unlock()
		m.Unmap()
		file.Close()
		return nil, fmt.Errorf("orders region empty")
	}
	orders := unsafe.Slice((*structs.Query)(unsafe.Pointer(&ordersData[0])), QueueCapacity)

	return &QueryQueue{
		file:   file,
		mmap:   m,
		header: header,
		orders: orders,
	}, nil
}

func (q *QueryQueue) Enqueue(order structs.Query) error {
	consumerTail := atomic.LoadUint64(&q.header.ConsumerTail)
	producerHead := atomic.LoadUint64(&q.header.ProducerHead)

	nextHead := producerHead + 1
	if nextHead-consumerTail > QueueCapacity {
		return fmt.Errorf("queue full - consumer too slow, backpressure at depth %d/%d",
			nextHead-consumerTail, QueueCapacity)
	}

	pos := producerHead % QueueCapacity
	q.orders[pos] = order

	// Publish after write; seq-cst store is sufficient
	atomic.StoreUint64(&q.header.ProducerHead, nextHead)
	return nil
}
func (q *QueryQueue) Dequeue() (*structs.Query, error) {
	producerHead := atomic.LoadUint64(&q.header.ProducerHead)
	consumerTail := atomic.LoadUint64(&q.header.ConsumerTail)

	if consumerTail == producerHead {
		return nil, nil
	}

	pos := consumerTail % QueueCapacity
	order := q.orders[pos]

	// Mark consumed; seq-cst store is sufficient
	atomic.StoreUint64(&q.header.ConsumerTail, consumerTail+1)
	return &order, nil
}

func (q *QueryQueue) Depth() uint64 {
	producerHead := atomic.LoadUint64(&q.header.ProducerHead)
	consumerTail := atomic.LoadUint64(&q.header.ConsumerTail)
	return producerHead - consumerTail
}

func (q *QueryQueue) Capacity() uint64 {
	return QueueCapacity
}

func (q *QueryQueue) Flush() error {
	return q.mmap.Flush()
}

func (q *QueryQueue) Close() error {
	_ = q.mmap.Flush()
	_ = q.mmap.Unlock()
	if err := q.mmap.Unmap(); err != nil {
		_ = q.file.Close()
		return fmt.Errorf("failed to unmap: %w", err)
	}
	return q.file.Close()
}


