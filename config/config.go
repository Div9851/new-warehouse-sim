package config

type Config struct {
	NumAgents      int     `json:"numAgents"`
	LastTurn       int     `json:"lastTurn"`
	NewItemProb    float64 `json:"newItemProb"`
	NumIters       int     `json:"numIters"`
	MaxDepth       int     `json:"maxDepth"`
	ExpandThresh   int     `json:"expandThresh"`
	PickupReward   float64 `json:"pickupReward"`
	ClearReward    float64 `json:"clearReward"`
	Penalty        float64 `json:"penalty"`
	DiscountFactor float64 `json:"discountFactor"`
	DecayRate      float64 `json:"decayRate"`
	RandSeed       int64   `json:"randSeed"`
	EnableRequest  bool    `json:"enableRequest"`
}
