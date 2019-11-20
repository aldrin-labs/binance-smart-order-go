package strategies

import (
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
)

type  SmartOrder struct {
	model models.MongoStrategy
}

func NewSmartOrder(sm *SmartOrder) *SmartOrder {
	return nil
}

func (sm *SmartOrder) Init(){

}

func (sm* SmartOrder) Start(){

}

func (sm* SmartOrder) processEventLoop(){

}

func StartEventLoop() {
}