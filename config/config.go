package config

type Config struct {
	NumAgents        int     `json:"numAgents"`
	LastTurn         int     `json:"lastTurn"`
	NewItemProb      float64 `json:"newItemProb"`
	NumIters         int     `json:"numIters"`
	MaxDepth         int     `json:"maxDepth"`
	ExpandThresh     int     `json:"expandThresh"`
	PickupReward     float64 `json:"pickupReward"`
	ClearReward      float64 `json:"clearReward"`
	Penalty          float64 `json:"penalty"`
	DistanceBonus    float64 `json:"distanceBonus"`
	BonusLimit       float64 `json:"bonusLimit"`
	DiscountFactor   float64 `json:"discountFactor"`
	RandSeed         int64   `json:"randSeed"`
	EnableExchange   bool    `json:"enableExchange,omitempty"`
	RequestStrategy  string  `json:"requestStrategy,omitempty"`
	AcceptStrategy   string  `json:"acceptStrategy,omitempty"`
	NominateStrategy string  `json:"nominateStrategy,omitempty"`
}
