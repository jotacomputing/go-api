package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"
)

const queueFilePath = "/tmp/sex"
const order = queue.Order{
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
func initQueue() {
	fmt.Println("[INIT] Initializing shared memory queue...")

	q, err := queue.CreateQueue(queueFilePath)
	if err != nil {
		log.Fatalf("Failed to create queue: %v", err)
	}
	defer q.Close()

	fmt.Printf("[INIT] Queue initialized successfully\n")
	fmt.Printf("[INIT] Capacity: %d orders\n", q.Capacity())
	fmt.Printf("[INIT] Queue depth: %d\n", q.Depth())
	fmt.Printf("[INIT] File: %s (size: ~3.2 MB)\n", queueFilePath)

	// Create status queue too
	fmt.Println("\n[INIT] Initializing status feedback queue...")
	statusQ, err := queue.CreateQueue(queueFilePath + "_status")
	if err != nil {
		log.Fatalf("Failed to create status queue: %v", err)
	}
	defer statusQ.Close()

	fmt.Printf("[INIT] Status queue initialized successfully\n")
	fmt.Printf("[INIT] File: %s_status (size: ~3.2 MB)\n", queueFilePath)
}

// sendOrder sends a single order
func sendOrder(order ShmOrder) {
	q, err := queue.OpenQueue(queueFilePath)
	if err != nil {
		log.Fatalf("Failed to open queue: %v", err)
	}
	defer q.Close()

	// order := queue.Order{
	// 	order_id:   3, //uint64
	// 	price: 		12000, //uint64
	// 	timestamp: 	uint64(time.Now().UnixNano()),
	// 	user_id:  	1001,
	// 	shares_qty: 16,
	// 	symbol:    	0,
	// 	side: 		0,
	// 	order_type: 0,
	// 	status:    	0, // pending
	// }

	if err := q.Enqueue(order); err != nil {
		log.Fatalf("Failed to enqueue: %v", err)
	}

	fmt.Printf("[COMM] Single order sent successfully\n")
	fmt.Printf("       OrderID: %d\n", order.OrderID)
	fmt.Printf("       Symbol: %s\n", string(order.Symbol))
	fmt.Printf("       Qty: %d @ %d\n", order.Quantity, order.Price)
	fmt.Printf("       Queue depth: %d\n", q.Depth())
}
func main() {
}
