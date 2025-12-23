package queue

import (
	"fmt"
	"jotacomputing/go-api/utils"
	"log"
)

var (
	// Global queues - opened once at startup
	IncomingOrderQueue *Queue
	CancelOrderQueue   *CancelQueue
	QueriesQueue       *QueryQueue
)

// Initialize ALL queues at startup
func InitQueues() error {
	var err error

	// Open incoming order queue ONCE
	IncomingOrderQueue, err = OpenQueue(utils.IncomingOrderQueuePath)
	if err != nil {
		return fmt.Errorf("failed to open incoming order queue: %v", err)
	}

	// Open cancel order queue ONCE
	CancelOrderQueue, err = OpenCancelQueue(utils.CancelOrderQueuePath)
	if err != nil {
		return fmt.Errorf("failed to open cancel order queue: %v", err)
	}
	// Open queries queue ONCE
	QueriesQueue, err = OpenQueryQueue(utils.QueryQueuePath)
	if err != nil {
		return fmt.Errorf("failed to open query queue: %v", err)
	}

	log.Println("✅ All queues initialized successfully")
	return nil
}

// Close ALL queues on shutdown
func CloseQueues() {
	if IncomingOrderQueue != nil {
		IncomingOrderQueue.Close()
	}
	if CancelOrderQueue != nil {
		CancelOrderQueue.Close()
	}
}