package main

import (
	"fmt"
	"log"
	"time"
)

const queueFilePath = "/tmp/sex"
var order = Order{
	order_id:   3, //uint64
	price: 		12000, //uint64
	timestamp: 	uint64(time.Now().UnixNano()),
	user_id:  	1001,
	shares_qty: 16,
	symbol:    	0,
	side: 		0,
	order_type: 0,
	status:    	0, // pending
}
	
// initializes the queue and validates structure
func initQueue(filePath string) {
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
func sendOrder(filePath string, order *Order) {
	q, err := OpenQueue(filePath)
	if err != nil {
		log.Fatalf("Failed to open queue: %v", err)
	}
	defer q.Close()

	if err := q.Enqueue((*order)); err != nil {
		log.Fatalf("Failed to enqueue: %v", err)
	}

	fmt.Printf("[COMM] Single order sent successfully\n")
	fmt.Printf("       OrderID: %d\n", (*order).order_id)
	fmt.Printf("       Symbol: %s\n", string((*order).symbol))
	fmt.Printf("       Qty: %d @ %d\n", (*order).shares_qty, (*order).price)
	fmt.Printf("       Queue depth: %d\n", q.Depth())
}


// orderBatch sends given number of orders rapidly
func orderBatch(filePath string, orders *[]Order, size int, maxRetries int) {
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

func main() {
 	initQueue(queueFilePath)
	sendOrder(queueFilePath, &order)
}
