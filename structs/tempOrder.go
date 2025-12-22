package structs

import (
	"errors"
	"fmt"
)

type TempOrder struct {
	Order_id  uint64
	Price     uint64
	Timestamp uint64
	// then u32s (4-byte aligned)
	Shares_qty uint32
	// then u8s (1-byte aligned)
	Symbol     uint32
	Side       uint8 // 0=buy 1=sell
	Order_type uint8 // 0=market order 1=limit order
}

type TempOrderToBeCancelled struct {
	Order_id uint64
	Symbol   uint32
}

type TempQuery struct {
	Query_id   uint64
	Query_type uint8 // 0 -> get balance , 1 -> get holdings , 2 -> add user on login 
}

func (o *TempOrder) Validate() error {

	if o.Price == 0 && o.Order_type == 1 {
		// for a limit order, price is required
		return errors.New("price must be > 0 for limit orders")
	}

	if o.Shares_qty == 0 {
		return errors.New("shares_qty must be > 0")
	}

	if o.Symbol == 0 {
		return errors.New("symbol must be specified")
	}

	if o.Side != 0 && o.Side != 1 {
		return fmt.Errorf("side must be 0 (buy) or 1 (sell), got %d", o.Side)
	}

	if o.Order_type != 0 && o.Order_type != 1 {
		return fmt.Errorf("order_type must be 0 (market) or 1 (limit), got %d", o.Order_type)
	}

	// Optional: timestamp > 0, or within some window, etc.
	if o.Timestamp == 0 {
		return errors.New("timestamp must be non-zero")
	}

	return nil
}
