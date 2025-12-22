package queue

import (
	"fmt"
	"os"
	"sync/atomic"
	"unsafe"
	"github.com/edsrzf/mmap-go"
	"jotacomputing/go-api/structs"
	"time"
	"log"
)


type QueueHeader struct {
	ProducerHead uint64   // Offset 0 4 byte interger 
	_pad1        [56]byte // Padding to cache line
	ConsumerTail uint64   // Offset 64
	_pad2        [56]byte // Padding
	Magic        uint32   // Offset 128
	Capacity     uint32   // Offset 132
}

const (
	QueueMagic    = 0xDEADBEEF
	QueueCapacity = 65536 // !!IMP: match Rust
	OrderSize     = unsafe.Sizeof(structs.Order{})
	HeaderSize    = unsafe.Sizeof(QueueHeader{})
	TotalSize     = HeaderSize + (QueueCapacity * OrderSize)
)

type Queue struct {
	file   *os.File
	mmap   mmap.MMap   // this is the array of bytes wich we will use to read and write 
	header *QueueHeader
	orders []structs.Order
}



	
// initializes the queue and validates structure
func InitQueue(filePath string) {
	fmt.Println("[INIT] Initializing shared memory queue...")

	q, err := CreateQueue(filePath)
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

// sendOrder sends a single order
func SendOrder(filePath string, order *structs.Order) {
	q, err := OpenQueue(filePath)
	if err != nil {
		log.Fatalf("Failed to open queue: %v", err)
	}
	defer q.Close()

	if err := q.Enqueue((*order)); err != nil {
		log.Fatalf("Failed to enqueue: %v", err)
	}

	fmt.Printf("[COMM] Single order sent successfully\n")
	fmt.Printf("       OrderID: %d\n", (*order).Order_id)
	fmt.Printf("       Symbol: %d\n", (*order).Symbol)
	fmt.Printf("       Qty: %d @ %d\n", (*order).Shares_qty, (*order).Price)
	fmt.Printf("       Queue depth: %d\n", q.Depth())
}



// orderBatch sends given number of orders rapidly
func OrderBatch(filePath string, orders *[]structs.Order, size int, maxRetries int) {
	fmt.Println("[COMM] Sending batch of %v orders...", size)

	q, err := OpenQueue(filePath)
	if err != nil {
		log.Fatalf("Failed to open queue: %v", err)
	}
	defer q.Close()

	startTime := time.Now()
	successCount := 0
	backpressureCount := 0

	for i := 1; i <= size; i++ {
		order := (*orders)[i]
		retries := 0

		for {
			if err := q.Enqueue(order); err == nil {
				successCount++
				break
			} else if retries < maxRetries {
				backpressureCount++
				retries++
				time.Sleep(time.Duration(1<<uint(retries)) * time.Millisecond) // exponantial backoff
			} else {
				log.Printf("Failed to enqueue order %d after retries", i)
				break
			}
		}

		// Progress indicator
		if i%1000 == 0 {
			elapsed := time.Since(startTime).Seconds()
			throughput := float64(i) / elapsed
			fmt.Printf("[TEST] Progress: %d/%d orders (%.0f orders/sec), depth: %d\n",
				i, size, throughput, q.Depth())
		}
	}

	elapsed := time.Since(startTime).Seconds()
	throughput := float64(successCount) / elapsed

	fmt.Printf("\n[COMM] Batch complete\n")
	fmt.Printf("       Sent: %d orders\n", successCount)
	fmt.Printf("       Backpressure events: %d\n", backpressureCount)
	fmt.Printf("       Time: %.2fs\n", elapsed)
	fmt.Printf("       Throughput: %.0f orders/sec\n", throughput)
	fmt.Printf("       Queue depth: %d\n", q.Depth())
}

func CreateQueue(filePath string) (*Queue, error) {
	_ = os.Remove(filePath)

	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	// set the size of the file
	if err := file.Truncate(int64(TotalSize)); err != nil {
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
	header := (*QueueHeader)(unsafe.Pointer(&m[0]))
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

	ordersData := m[int(HeaderSize):int(TotalSize)]
	if len(ordersData) == 0 {
		m.Unlock()
		m.Unmap()
		file.Close()
		return nil, fmt.Errorf("orders region empty")
	}
	orders := unsafe.Slice((*structs.Order)(unsafe.Pointer(&ordersData[0])), QueueCapacity)

	return &Queue{
		file:   file,
		mmap:   m,
		header: header,
		orders: orders,
	}, nil

}

// open queue from file on disk and return *Queue mmap-ed
func OpenQueue(filePath string) (*Queue, error) {
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
	if stat.Size() != int64(TotalSize) {
		file.Close()
		return nil, fmt.Errorf("invalid file size: got %d, expected %d", stat.Size(), int64(TotalSize))
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
	header := (*QueueHeader)(unsafe.Pointer(&m[0]))
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

	ordersData := m[int(HeaderSize):int(TotalSize)]
	if len(ordersData) == 0 {
		m.Unlock()
		m.Unmap()
		file.Close()
		return nil, fmt.Errorf("orders region empty")
	}
	orders := unsafe.Slice((*structs.Order)(unsafe.Pointer(&ordersData[0])), QueueCapacity)

	return &Queue{
		file:   file,
		mmap:   m,
		header: header,
		orders: orders,
	}, nil
}

func (q *Queue) Enqueue(order structs.Order) error {
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

func (q *Queue) Dequeue() (*structs.Order, error) {
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

func (q *Queue) Depth() uint64 {
	producerHead := atomic.LoadUint64(&q.header.ProducerHead)
	consumerTail := atomic.LoadUint64(&q.header.ConsumerTail)
	return producerHead - consumerTail
}

func (q *Queue) Capacity() uint64 {
	return QueueCapacity
}

func (q *Queue) Flush() error {
	return q.mmap.Flush()
}

func (q *Queue) Close() error {
	_ = q.mmap.Flush()
	_ = q.mmap.Unlock()
	if err := q.mmap.Unmap(); err != nil {
		_ = q.file.Close()
		return fmt.Errorf("failed to unmap: %w", err)
	}
	return q.file.Close()
}
