package agent

type Action int

const (
	ACTION_UP Action = iota
	ACTION_DOWN
	ACTION_LEFT
	ACTION_RIGHT
	ACTION_STAY
	ACTION_PICKUP
	ACTION_CLEAR
	ACTION_UNKNOWN
)

func (action Action) ToStr() string {
	switch action {
	case ACTION_UP:
		return "UP"
	case ACTION_DOWN:
		return "DOWN"
	case ACTION_LEFT:
		return "LEFT"
	case ACTION_RIGHT:
		return "RIGHT"
	case ACTION_STAY:
		return "STAY"
	case ACTION_PICKUP:
		return "PICKUP"
	case ACTION_CLEAR:
		return "CLEAR"
	}
	return "UNKNOWN"
}

type Actions []Action
