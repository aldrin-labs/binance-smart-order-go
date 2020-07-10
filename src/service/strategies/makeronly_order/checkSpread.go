package makeronly_order

import "context"

func(po *MakerOnlyOrder) checkSpread(ctx context.Context, args ...interface{}) bool {
	return true
}