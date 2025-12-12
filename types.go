package main

type ShmOrder struct {
	order_id uint64
	price uint64
	timestamp uint64
	user_id uint64
	// then u32s (4-byte aligned)
	shares_qty uint32
	// then u8s (1-byte aligned)
	symbol uint32
	side uint8 // 0=buy 1=sell
	order_type uint8 // 0=market order 1=limit order
	status uint8 // O=pending 1=filled 2=rejected
}
