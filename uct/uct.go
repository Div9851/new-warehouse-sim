package uct

import (
	"math"
	"math/rand"
	"sync"

	"github.com/Div9851/new-warehouse-sim/agentaction"
	"github.com/Div9851/new-warehouse-sim/agentstate"
	"github.com/Div9851/new-warehouse-sim/config"
	"github.com/Div9851/new-warehouse-sim/mapdata"
)

type Node struct {
	CumReward  []float64
	SelectCnt  []float64
	TotalCnt   float64
	RolloutCnt int
}

func NewNode() *Node {
	return &Node{
		CumReward:  make([]float64, agentaction.COUNT),
		SelectCnt:  make([]float64, agentaction.COUNT),
		TotalCnt:   0,
		RolloutCnt: 0,
	}
}

func (node *Node) UCB1(action agentaction.Action) float64 {
	if node.SelectCnt[action] == 0 {
		return math.Inf(1)
	}
	score := node.CumReward[action]/node.SelectCnt[action] + math.Sqrt(2*math.Log(node.TotalCnt)/node.SelectCnt[action])
	return score
}

func (node *Node) Select(actions agentaction.Actions) agentaction.Action {
	selected := agentaction.COUNT
	maxScore := math.Inf(-1)
	for _, action := range actions {
		score := node.UCB1(action)
		if maxScore < score {
			maxScore = score
			selected = action
		}
	}
	return selected
}

func (node *Node) GetBestAction(actions agentaction.Actions) agentaction.Action {
	selected := agentaction.COUNT
	maxScore := math.Inf(-1)
	for _, action := range actions {
		if maxScore < node.SelectCnt[action] {
			maxScore = node.SelectCnt[action]
			selected = action
		}
	}
	return selected
}

func (node *Node) Reset() {
	for i := range node.CumReward {
		node.CumReward[i] = 0
		node.SelectCnt[i] = 0
	}
	node.TotalCnt = 0
	node.RolloutCnt = 0
}

type Planner struct {
	Id          int
	Nodes       []map[agentstate.State]*Node // [depth][state]
	MapData     *mapdata.MapData
	RandGen     *rand.Rand
	NodePool    *sync.Pool
	NewItemProb float64
}

func New(id int, mapData *mapdata.MapData, randGen *rand.Rand, nodePool *sync.Pool, newItemProb float64) *Planner {
	return &Planner{
		Id:          id,
		Nodes:       nil,
		MapData:     mapData,
		RandGen:     randGen,
		NodePool:    nodePool,
		NewItemProb: newItemProb,
	}
}

func GetValidActions(state agentstate.State, items map[mapdata.Pos]int, mapData *mapdata.MapData) agentaction.Actions {
	actions := make(agentaction.Actions, len(mapData.ValidActions[state.Pos.R][state.Pos.C]))
	copy(actions, mapData.ValidActions[state.Pos.R][state.Pos.C])
	if !state.HasItem && items[state.Pos] > 0 {
		actions = append(actions, agentaction.PICKUP)
	}
	if state.HasItem && state.Pos == mapData.DepotPos {
		actions = append(actions, agentaction.CLEAR)
	}
	return actions
}

func Greedy(id int, states agentstate.States, items []map[mapdata.Pos]int, targetPos []mapdata.Pos, mapData *mapdata.MapData, randGen *rand.Rand) agentaction.Action {
	state := states[id]
	validActions := GetValidActions(state, items[id], mapData)
	if targetPos[id] == state.Pos {
		targetPos[id] = mapdata.NonePos
	}
	if targetPos[id] == mapdata.NonePos {
		if state.HasItem {
			if state.Pos == mapData.DepotPos {
				return agentaction.CLEAR
			}
			targetPos[id] = mapData.DepotPos
		} else {
			if items[id][state.Pos] > 0 {
				return agentaction.PICKUP
			}
			d := math.MaxInt
			for pos, itemNum := range items[id] {
				if itemNum > 0 && d > mapData.MinDist[state.Pos.R][state.Pos.C][pos.R][pos.C] {
					d = mapData.MinDist[state.Pos.R][state.Pos.C][pos.R][pos.C]
					targetPos[id] = pos
				}
			}
			// アイテムのある頂点がない場合、ランダムに行動
			if targetPos[id] == mapdata.NonePos {
				return validActions[randGen.Intn(len(validActions))]
			}
		}
	}
	optimal := agentaction.Actions{}
	for _, action := range validActions {
		nxtPos := mapData.NextPos[state.Pos.R][state.Pos.C][action]
		if mapData.MinDist[state.Pos.R][state.Pos.C][targetPos[id].R][targetPos[id].C] > mapData.MinDist[nxtPos.R][nxtPos.C][targetPos[id].R][targetPos[id].C] {
			optimal = append(optimal, action)
		}
	}
	return optimal[randGen.Intn(len(optimal))]
}

func (planner *Planner) GetBestAction(id int, curState agentstate.State, items map[mapdata.Pos]int) agentaction.Action {
	node := planner.Nodes[0][curState]
	validActions := GetValidActions(curState, items, planner.MapData)
	return node.GetBestAction(validActions)
}

func (planner *Planner) Update(turn int, curStates agentstate.States, items []map[mapdata.Pos]int) {
	targetPos := make([]mapdata.Pos, config.NumAgents)
	itemsCopy := make([]map[mapdata.Pos]int, config.NumAgents)
	for i := 0; i < config.NumAgents; i++ {
		targetPos[i] = mapdata.NonePos
		itemsCopy[i] = make(map[mapdata.Pos]int)
		for pos, itemNum := range items[i] {
			itemsCopy[i][pos] = itemNum
		}
	}

	planner.update(turn, 0, curStates, itemsCopy, false, targetPos)
}

func (planner *Planner) update(turn int, depth int, curStates agentstate.States, items []map[mapdata.Pos]int, rollout bool, targetPos []mapdata.Pos) float64 {
	if turn == config.LastTurn || depth == config.MaxDepth {
		return 0
	}
	state := curStates[planner.Id]
	actions := make(agentaction.Actions, config.NumAgents)
	nxtRollout := rollout
	var curNode *Node
	if !rollout {
		if len(planner.Nodes) <= depth {
			planner.Nodes = append(planner.Nodes, make(map[agentstate.State]*Node))
		}
		if node, exist := planner.Nodes[depth][state]; exist {
			curNode = node
		} else {
			curNode = planner.NodePool.Get().(*Node)
			planner.Nodes[depth][state] = curNode
		}
		if curNode.RolloutCnt < config.ExpandThresh {
			curNode.RolloutCnt++
			nxtRollout = true
		}
	}
	if nxtRollout {
		actions[planner.Id] = Greedy(planner.Id, curStates, items, targetPos, planner.MapData, planner.RandGen)
	} else {
		// UCB アルゴリズムに従って行動選択
		validActions := GetValidActions(state, items[planner.Id], planner.MapData)
		actions[planner.Id] = curNode.Select(validActions)
	}
	for i := 0; i < config.NumAgents; i++ {
		if i != planner.Id {
			actions[i] = Greedy(i, curStates, items, targetPos, planner.MapData, planner.RandGen)
		}
	}
	free := make([]bool, config.NumAgents)
	free[planner.Id] = nxtRollout
	routes := make([][]mapdata.Pos, config.NumAgents)
	nxtStates, rewards, _ := agentstate.Next(curStates, actions, free, items, routes, planner.MapData, planner.RandGen, planner.NewItemProb)
	cumReward := planner.update(turn+1, depth+1, nxtStates, items, nxtRollout, targetPos)
	cumReward = rewards[planner.Id] + config.DiscountFactor*cumReward
	if !rollout {
		curNode.TotalCnt++
		curNode.SelectCnt[actions[planner.Id]]++
		curNode.CumReward[actions[planner.Id]] += cumReward
	}
	return cumReward
}

func (planner *Planner) Free() {
	for i := range planner.Nodes {
		for _, node := range planner.Nodes[i] {
			node.Reset()
			planner.NodePool.Put(node)
		}
	}
}
