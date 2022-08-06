package mcts

import (
	"math"
	"math/rand"
	"sort"

	"github.com/Div9851/new-warehouse-sim/agent"
	"github.com/Div9851/new-warehouse-sim/config"
	"github.com/Div9851/new-warehouse-sim/mapdata"
)

type ActionScore struct {
	Action agent.Action
	Score  float64
}

type ActionScores []ActionScore

func (scores ActionScores) Len() int {
	return len(scores)
}

func (scores ActionScores) Swap(i, j int) {
	scores[i], scores[j] = scores[j], scores[i]
}

func (scores ActionScores) Less(i, j int) bool {
	return scores[i].Score < scores[j].Score
}

type Node struct {
	Turn       int
	Depth      int
	Agents     agent.Agents
	PlannerId  int
	MapData    mapdata.MapData
	AllPos     []mapdata.Pos
	Reserved   map[mapdata.PosTurn]int
	RandGen    *rand.Rand
	Childs     map[agent.Action][]*Node
	ImmReward  map[agent.Action][]float64
	CumReward  map[agent.Action]float64
	ItemsDiff  map[agent.Action][]agent.ItemsDiff
	TotalCnt   int
	SelectCnt  map[agent.Action]int
	RolloutCnt int
}

func New(turn int, depth int, agents agent.Agents, plannerId int, mapData mapdata.MapData, allPos []mapdata.Pos, reserved map[mapdata.PosTurn]int, randGen *rand.Rand) *Node {
	node := Node{
		Turn:       turn,
		Depth:      depth,
		Agents:     agents,
		PlannerId:  plannerId,
		MapData:    mapData,
		AllPos:     allPos,
		Reserved:   reserved,
		RandGen:    randGen,
		Childs:     make(map[agent.Action][]*Node),
		ImmReward:  make(map[agent.Action][]float64),
		CumReward:  make(map[agent.Action]float64),
		ItemsDiff:  make(map[agent.Action][]agent.ItemsDiff),
		TotalCnt:   0,
		SelectCnt:  make(map[agent.Action]int),
		RolloutCnt: 0,
	}
	return &node
}

func MCTS(node *Node, items agent.Items) agent.Action {
	for i := 0; i < config.NumRollouts; i++ {
		var itemsClone agent.Items
		for _, item := range items {
			itemClone := make(agent.Item)
			for k, v := range item {
				itemClone[k] = v
			}
			itemsClone = append(itemsClone, itemClone)
		}
		Update(node, itemsClone)
	}
	chosen, _ := BestAction(node)
	return chosen
}

func Update(node *Node, items agent.Items) float64 {
	if node.Turn == config.LastTurn || node.Depth == config.MaxDepth {
		return 0
	}
	if node.RolloutCnt < config.ExpandThresh {
		node.RolloutCnt++
		return Rollout(node, items)
	}
	planner := node.Agents[node.PlannerId]
	actions := PossibleActions(node.PlannerId, planner.Pos.R, planner.Pos.C, node.Turn, node.Reserved, node.MapData)
	if len(actions) == 0 {
		return 0
	}
	if actions[0] == agent.ACTION_STAY {
		if v, ok := items[planner.Id][planner.Pos]; ok && v > 0 && !planner.HasItem {
			actions = append(actions, agent.ACTION_PICKUP)
		}
		if r, c := planner.Pos.R, planner.Pos.C; node.MapData[r][c] == 'D' && planner.HasItem {
			actions = append(actions, agent.ACTION_CLEAR)
		}
	}
	chosen := Select(node, actions)
	var child *Node
	var cumReward float64
	var diff agent.ItemsDiff
	if childs, ok := node.Childs[chosen]; ok && len(childs) >= config.MaxChilds {
		r := node.RandGen.Intn(len(childs))
		child = childs[r]
		cumReward = node.ImmReward[chosen][r]
		diff = node.ItemsDiff[chosen][r]
	} else {
		actions := agent.Actions{}
		for i := 0; i < config.NumAgents; i++ {
			if i == planner.Id {
				actions = append(actions, chosen)
			} else {
				actions = append(actions, BFS(i, node, items))
			}
		}
		nxtAgents, rewards, _, itemsDiff := node.Agents.Next(actions, items, node.MapData, node.AllPos, node.RandGen)
		child = New(
			node.Turn+1,
			node.Depth+1,
			nxtAgents,
			node.PlannerId,
			node.MapData,
			node.AllPos,
			node.Reserved,
			node.RandGen)
		immReward := rewards[node.PlannerId]
		cumReward = immReward
		diff = itemsDiff
		node.Childs[chosen] = append(node.Childs[chosen], child)
		node.ImmReward[chosen] = append(node.ImmReward[chosen], immReward)
		node.ItemsDiff[chosen] = append(node.ItemsDiff[chosen], itemsDiff)
	}
	for i := range diff {
		for k, v := range diff[i] {
			items[i][k] += v
		}
	}
	cumReward += config.DiscountFactor * Update(child, items)
	node.TotalCnt++
	node.SelectCnt[chosen]++
	node.CumReward[chosen] += cumReward
	return cumReward
}

func BestAction(node *Node) (agent.Action, bool) {
	var scores ActionScores
	for action, reward := range node.CumReward {
		score := reward / float64(node.SelectCnt[action])
		scores = append(scores, ActionScore{Action: action, Score: score})
	}
	if len(scores) == 0 {
		return agent.ACTION_STAY, false
	}
	sort.Sort(sort.Reverse(scores))
	return scores[0].Action, true
}

func Select(node *Node, actions agent.Actions) agent.Action {
	var scores ActionScores
	for _, action := range actions {
		var score float64
		if node.SelectCnt[action] == 0 {
			score = math.Inf(0)
		} else {
			score = node.CumReward[action]/float64(node.SelectCnt[action]) + math.Sqrt(config.UCBParam*math.Log(float64(node.TotalCnt))/float64(node.SelectCnt[action]))
		}
		scores = append(scores, ActionScore{Action: action, Score: score})
	}
	sort.Sort(sort.Reverse(scores))
	return scores[0].Action
}

func Rollout(node *Node, items agent.Items) float64 {
	planner := node.Agents[node.PlannerId]
	turn := node.Turn
	que := []mapdata.PosTurn{}
	visited := make(map[mapdata.PosTurn]struct{})
	s := mapdata.PosTurn{R: planner.Pos.R, C: planner.Pos.C, Turn: node.Turn}
	que = append(que, s)
	visited[s] = struct{}{}
	d := config.RolloutLimit
	for len(que) > 0 {
		cur := que[0]
		que = que[1:]
		if cur.Turn-turn > config.RolloutLimit {
			break
		}
		curPos := mapdata.Pos{R: cur.R, C: cur.C}
		if v, ok := items[planner.Id][curPos]; ok && v > 0 && !planner.HasItem {
			d = cur.Turn - turn
			break
		}
		if r, c := curPos.R, curPos.C; node.MapData[r][c] == 'D' && planner.HasItem {
			d = cur.Turn - turn
			break
		}
		actions := PossibleActions(planner.Id, cur.R, cur.C, cur.Turn, node.Reserved, node.MapData)
		for _, action := range actions {
			nxtPos := agent.NextPosSA(mapdata.Pos{R: cur.R, C: cur.C}, action, node.MapData)
			nxt := mapdata.PosTurn{R: nxtPos.R, C: nxtPos.C, Turn: cur.Turn + 1}
			if _, ok := visited[nxt]; !ok {
				que = append(que, nxt)
				visited[nxt] = struct{}{}
			}
		}
	}
	return config.Reward * math.Pow(config.DiscountFactor, float64(d))
}

func PossibleActions(plannerId int, r int, c int, turn int, reserved map[mapdata.PosTurn]int, mapData mapdata.MapData) agent.Actions {
	h, w := mapData.Size()
	actions := agent.Actions{}
	if resId, exist := reserved[mapdata.PosTurn{R: r, C: c, Turn: turn + 1}]; !exist || resId == plannerId {
		actions = append(actions, agent.ACTION_STAY)
	}
	if r > 0 && mapData[r-1][c] != '#' {
		if resId, exist := reserved[mapdata.PosTurn{R: r - 1, C: c, Turn: turn + 1}]; !exist || resId == plannerId {
			actions = append(actions, agent.ACTION_UP)
		}
	}
	if r+1 < h && mapData[r+1][c] != '#' {
		if resId, exist := reserved[mapdata.PosTurn{R: r + 1, C: c, Turn: turn + 1}]; !exist || resId == plannerId {
			actions = append(actions, agent.ACTION_DOWN)
		}
	}
	if c > 0 && mapData[r][c-1] != '#' {
		if resId, exist := reserved[mapdata.PosTurn{R: r, C: c - 1, Turn: turn + 1}]; !exist || resId == plannerId {
			actions = append(actions, agent.ACTION_LEFT)
		}
	}
	if c+1 < w && mapData[r][c+1] != '#' {
		if resId, exist := reserved[mapdata.PosTurn{R: r, C: c + 1, Turn: turn + 1}]; !exist || resId == plannerId {
			actions = append(actions, agent.ACTION_RIGHT)
		}
	}
	return actions
}

func BFS(plannerId int, node *Node, items agent.Items) agent.Action {
	que := []mapdata.PosTurn{}
	pos := node.Agents[plannerId].Pos
	s := mapdata.PosTurn{R: pos.R, C: pos.C, Turn: node.Turn}
	t := s
	que = append(que, s)
	prev := make(map[mapdata.PosTurn]mapdata.PosTurn)
	last := make(map[mapdata.PosTurn]agent.Action)
	for len(que) > 0 {
		cur := que[0]
		que = que[1:]
		if cur.Turn-node.Turn > config.BFSLimit {
			break
		}
		if node.Reserved[cur] == plannerId {
			t = cur
			break
		}
		actions := PossibleActions(plannerId, cur.R, cur.C, cur.Turn, node.Reserved, node.MapData)
		for _, action := range actions {
			nxtPos := agent.NextPosSA(mapdata.Pos{R: cur.R, C: cur.C}, action, node.MapData)
			nxt := mapdata.PosTurn{R: nxtPos.R, C: nxtPos.C, Turn: node.Turn + 1}
			if _, exist := prev[nxt]; !exist {
				prev[nxt] = cur
				last[nxt] = action
				que = append(que, nxt)
			}
		}
	}
	chosen := agent.ACTION_UNKNOWN
	for t != s {
		chosen = last[t]
		t = prev[t]
	}
	if chosen == agent.ACTION_UNKNOWN {
		actions := PossibleActions(plannerId, pos.R, pos.C, node.Turn, node.Reserved, node.MapData)
		if len(actions) == 0 {
			chosen = agent.ACTION_STAY
		} else {
			chosen = actions[node.RandGen.Intn(len(actions))]
		}
	}
	return chosen
}
