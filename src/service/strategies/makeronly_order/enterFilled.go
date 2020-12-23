package makeronly_order

import "context"

func (mo *MakerOnlyOrder) enterFilled(ctx context.Context, args ...interface{}) error {
	state := mo.Strategy.GetModel().State
	state.State = Filled
	mo.StateMgmt.UpdateState(mo.Strategy.GetModel().ID, state)

	return nil
}
