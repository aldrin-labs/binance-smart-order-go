package makeronly_order

import (
	"context"
)

func(po *MakerOnlyOrder) checkOrPlaceOrders(ctx context.Context, args ...interface{}) bool {
    // if there is an order in state
	// if no go to place order
	// wait N sec
	// if not filled
	// try cancel
	// if filled
	// if success
	// place order
	// if no then order was filled so thats it
	return true
}
