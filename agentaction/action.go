package agentaction

type Action int

const (
	UP Action = iota
	DOWN
	LEFT
	RIGHT
	STAY
	PICKUP
	CLEAR
	UNKNOWN
)

func (action Action) ToStr() string {
	switch action {
	case UP:
		return "UP"
	case DOWN:
		return "DOWN"
	case LEFT:
		return "LEFT"
	case RIGHT:
		return "RIGHT"
	case STAY:
		return "STAY"
	case PICKUP:
		return "PICKUP"
	case CLEAR:
		return "CLEAR"
	}
	return "UNKNOWN"
}

type Actions []Action
