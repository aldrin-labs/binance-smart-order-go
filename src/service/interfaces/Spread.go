package interfaces

type SpreadData struct {
	BestAsk float64 `json:"bestAsk,float"`
	BestBid float64 `json:"bestBid,float"`
}