package structs


type Order struct {
	Order_id uint64
	Price uint64
	Timestamp uint64
	User_id uint64
	// then u32s (4-byte aligned)
	Shares_qty uint32
	// then u8s (1-byte aligned)
	Symbol uint32
	Side uint8 // 0=buy 1=sell
	Order_type uint8 // 0=market order 1=limit order
	Status uint8 // O=pending 1=filled 2=rejected
}

type OrderToBeCancelled struct {
	Order_id uint64
	User_id uint64
	Symbol uint32
}

type Query struct {
	Query_id uint64
	User_id uint64
	Query_type uint8 // 0 -> get balance , 1 -> get holdings , 2 -> add user on login 

}
