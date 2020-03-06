package smart_order

import (
	"context"
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
	"time"
)

func (sm *SmartOrder) enterTrailingEntry(ctx context.Context, args ...interface{}) error {
	sm.Strategy.GetModel().State.State = TrailingEntry
	sm.StateMgmt.UpdateState(sm.Strategy.GetModel().ID, sm.Strategy.GetModel().State)
	return nil
}

func (sm *SmartOrder) checkTrailingEntry(ctx context.Context, args ...interface{}) bool {
	//isWaitingForOrder, ok := sm.IsWaitingForOrder.Load(TrailingEntry)
	//if ok && isWaitingForOrder.(bool) {
	//	return false
	//}
	currentOHLCV := args[0].(interfaces.OHLCV)
	edgePrice := sm.Strategy.GetModel().State.TrailingEntryPrice
	activateTrailing := false
	if edgePrice == 0 {
		println("edgePrice=0")
		sm.Strategy.GetModel().State.TrailingEntryPrice = currentOHLCV.Close
		activateTrailing = true
	}
	// println(currentOHLCV.Close, edgePrice, currentOHLCV.Close/edgePrice-1)
	deviation := sm.Strategy.GetModel().Conditions.EntryOrder.EntryDeviation / sm.Strategy.GetModel().Conditions.Leverage
	side := sm.Strategy.GetModel().Conditions.EntryOrder.Side
	isSpotMarketEntry := sm.Strategy.GetModel().Conditions.MarketType == 0 && sm.Strategy.GetModel().Conditions.EntryOrder.OrderType == "market"
	switch side {
	case "buy":
		if  !isSpotMarketEntry && (activateTrailing || currentOHLCV.Close < edgePrice) {
			edgePrice = sm.Strategy.GetModel().State.TrailingEntryPrice
			go sm.placeTrailingOrder(currentOHLCV.Close, time.Now().UnixNano(), 0, side, true, TrailingEntry)
		}
		if isSpotMarketEntry && ( activateTrailing || (currentOHLCV.Close/edgePrice-1)*100 >= deviation ) {
			return true
		}
		break
	case "sell":
		if !isSpotMarketEntry && (activateTrailing || currentOHLCV.Close > edgePrice) {
			go sm.placeTrailingOrder(currentOHLCV.Close, time.Now().UnixNano(), 0, side, true, TrailingEntry)
		}
		if isSpotMarketEntry && (activateTrailing || (1-currentOHLCV.Close/edgePrice)*100 >= deviation) {
			return true
		}
		break
	}
	return false
}

func (sm *SmartOrder) checkTrailingProfit(ctx context.Context, args ...interface{}) bool {
	//isWaitingForOrder, ok := sm.IsWaitingForOrder.Load(TakeProfit)
	//if ok && isWaitingForOrder.(bool) {
	//	return false
	//}
	currentOHLCV := args[0].(interfaces.OHLCV)
	model := sm.Strategy.GetModel()
	side := model.Conditions.EntryOrder.Side
	switch side {
	case "buy":
		for i, target := range model.Conditions.ExitLevels {
			isTrailingTarget := target.ActivatePrice > 0
			if isTrailingTarget {
				isActivated := i < len(model.State.TrailingExitPrices)
				deviation := target.EntryDeviation / 100 / model.Conditions.Leverage
				activateDeviation := target.ActivatePrice / 100 / model.Conditions.Leverage

				activatePrice := target.ActivatePrice
				if target.Type == 1 {
					activatePrice = model.State.EntryPrice * (1 + activateDeviation)
				}
				didCrossActivatePrice := currentOHLCV.Close >= activatePrice

				if !isActivated && didCrossActivatePrice {
					model.State.TrailingExitPrices = append(model.State.TrailingExitPrices, currentOHLCV.Close)
					return false
				} else if !isActivated && !didCrossActivatePrice {
					return false
				}
				edgePrice := model.State.TrailingExitPrices[i]

				if currentOHLCV.Close > edgePrice {
					model.State.TrailingExitPrices[i] = currentOHLCV.Close
					edgePrice = model.State.TrailingExitPrices[i]

					go sm.placeTrailingOrder(edgePrice, time.Now().UnixNano(), i, side, false, TakeProfit)
				}

				deviationFromEdge := (edgePrice/currentOHLCV.Close - 1) * 100
				if deviationFromEdge > deviation {
					isSpot := model.Conditions.MarketType == 0
					isSpotMarketOrder := target.OrderType == "market" && isSpot
					if isSpotMarketOrder {
						model.State.State = TakeProfit
						sm.placeOrder(edgePrice, TakeProfit)

						return true
					}
				}
			}
		}
		break
	case "sell":
		for i, target := range sm.Strategy.GetModel().Conditions.ExitLevels {
			isTrailingTarget := target.ActivatePrice > 0
			if isTrailingTarget {
				isActivated := i < len(sm.Strategy.GetModel().State.TrailingExitPrices)
				deviation := target.EntryDeviation / 100 / sm.Strategy.GetModel().Conditions.Leverage
				activateDeviation := target.ActivatePrice / 100 / sm.Strategy.GetModel().Conditions.Leverage

				activatePrice := target.ActivatePrice
				if target.Type == 1 {
					activatePrice = sm.Strategy.GetModel().State.EntryPrice * (1 - activateDeviation)
				}
				didCrossActivatePrice := currentOHLCV.Close <= activatePrice

				if !isActivated && didCrossActivatePrice {
					sm.Strategy.GetModel().State.TrailingExitPrices = append(sm.Strategy.GetModel().State.TrailingExitPrices, currentOHLCV.Close)
					return false
				} else if !isActivated && !didCrossActivatePrice {
					return false
				}
				// isActivated
				edgePrice := sm.Strategy.GetModel().State.TrailingExitPrices[i]

				if currentOHLCV.Close < edgePrice {
					sm.Strategy.GetModel().State.TrailingExitPrices[i] = currentOHLCV.Close
					edgePrice = sm.Strategy.GetModel().State.TrailingExitPrices[i]

					go sm.placeTrailingOrder(edgePrice, time.Now().UnixNano(), i, side, false, TakeProfit)
				}

				deviationFromEdge := (currentOHLCV.Close/edgePrice - 1) * 100
				if deviationFromEdge > deviation {
					isSpot := sm.Strategy.GetModel().Conditions.MarketType == 0
					isSpotMarketOrder := target.OrderType == "market" && isSpot
					if isSpotMarketOrder {
						sm.Strategy.GetModel().State.State = TakeProfit
						sm.placeOrder(edgePrice, TakeProfit)

						return true
					}
				}
			}
		}
		break
	}

	return false
}

func (sm *SmartOrder) placeTrailingOrder(newTrailingPrice float64, trailingCheckAt int64, i int, entrySide string, isEntry bool, step string) {
	sm.Strategy.GetModel().State.TrailingCheckAt = trailingCheckAt
	time.Sleep(2 * time.Second)
	model := sm.Strategy.GetModel()
	edgePrice := model.State.TrailingEntryPrice
	if isEntry == false {
		if step != HedgeLoss {
			edgePrice = model.State.TrailingExitPrices[i]
		} else {
			edgePrice = model.State.TrailingHedgeExitPrice
		}
	}
	trailingDidnChange := trailingCheckAt == model.State.TrailingCheckAt
	newTrailingIncreaseProfits := (newTrailingPrice < edgePrice && !isEntry) || (newTrailingPrice > edgePrice && isEntry)
	priceRelation := edgePrice / newTrailingPrice
	if isEntry {
		priceRelation = newTrailingPrice / edgePrice
	}
	significantPercent := 0.016 * model.Conditions.Leverage
	trailingChangedALot := newTrailingIncreaseProfits && (significantPercent < (priceRelation-1)*100*model.Conditions.Leverage)
	if entrySide == "buy" {
		if !isEntry {
			priceRelation = newTrailingPrice / edgePrice
		} else {
			priceRelation = edgePrice / newTrailingPrice
		}
		newTrailingIncreaseProfits = (newTrailingPrice > edgePrice && !isEntry) || (newTrailingPrice < edgePrice && isEntry)
		trailingChangedALot = newTrailingIncreaseProfits && (significantPercent < (priceRelation-1)*100*model.Conditions.Leverage)
	}
	if trailingDidnChange || trailingChangedALot {
		if sm.Lock == false {
			sm.Lock = true
			model.State.TrailingEntryPrice = newTrailingPrice
			sm.placeOrder(-1, step)
			time.Sleep(3000 * time.Millisecond) // it will give some time for order execution, to avoid double send of orders
			sm.Lock = false
		}
	}
}
