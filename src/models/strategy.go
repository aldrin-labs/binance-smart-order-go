package models

// strategy db model
import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type RBAC struct {
	Id			primitive.ObjectID `json:"_id"`
	userId string
	portfolioId string
	accessLevel int32
}

type MongoSocial struct {
	sharedWith []primitive.ObjectID // [RBAC]
	isPrivate bool
}

type StrategyModel struct {
	Id			primitive.ObjectID `json:"_id"`
	Name     	string
	LastUpdate	int64
	SignalIds 	[] primitive.ObjectID
	OrderIds   	[] primitive.ObjectID
	OwnerId 	primitive.ObjectID
	Social	MongoSocial // {sharedWith: [RBAC]}
}