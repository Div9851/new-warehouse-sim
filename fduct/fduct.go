package fduct

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

func (node *Node) GetBestAction(actions agentaction.Actions) (agentaction.Action, float64) {
	selected := agentaction.COUNT
	maxScore := math.Inf(-1)
	for _, action := range actions {
		if maxScore < node.SelectCnt[action] {
			maxScore = node.SelectCnt[action]
			selected = action
		}
	}
	reward := node.CumReward[selected] / node.SelectCnt[selected]
	return selected, reward
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
	Nodes       [][]map[agentstate.State]*Node // [id][depth][state]
	MapData     *mapdata.MapData
	Config      *config.Config
	RandGen     *rand.Rand
	NodePool    *sync.Pool
	NewItemProb float64
}

func New(mapData *mapdata.MapData, config *config.Config, randGen *rand.Rand, nodePool *sync.Pool, newItemProb float64) *Planner {
	nodes := make([][]map[agentstate.State]*Node, config.NumAgents)
	return &Planner{
		Nodes:       nodes,
		MapData:     mapData,
		Config:      config,
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

func (planner *Planner) GetBestAction(id int, curState agentstate.State, items map[mapdata.Pos]int) (agentaction.Action, float64) {
	node := planner.Nodes[id][0][curState]
	validActions := GetValidActions(curState, items, planner.MapData)
	return node.GetBestAction(validActions)
}

func (planner *Planner) Update(turn int, curStates agentstate.States, items []map[mapdata.Pos]int, iterIdx int) {
	rollout := make([]bool, planner.Config.NumAgents)
	targetPos := make([]mapdata.Pos, planner.Config.NumAgents)
	itemsCopy := make([]map[mapdata.Pos]int, planner.Config.NumAgents)
	for i := 0; i < planner.Config.NumAgents; i++ {
		targetPos[i] = mapdata.NonePos
		itemsCopy[i] = make(map[mapdata.Pos]int)
		for pos, itemNum := range items[i] {
			itemsCopy[i][pos] = itemNum
		}
	}
	planner.update(turn, 0, curStates, itemsCopy, rollout, targetPos, iterIdx)
}

func (planner *Planner) update(turn int, depth int, curStates agentstate.States, items []map[mapdata.Pos]int, rollout []bool, targetPos []mapdata.Pos, iterIdx int) []float64 {
	if turn == planner.Config.LastTurn || depth == planner.Config.MaxDepth {
		return make([]float64, planner.Config.NumAgents)
	}
	actions := make(agentaction.Actions, planner.Config.NumAgents)
	nxtRollout := make([]bool, planner.Config.NumAgents)
	copy(nxtRollout, rollout)
	nodes := make([]*Node, planner.Config.NumAgents)
	for i, state := range curStates {
		if !rollout[i] {
			if len(planner.Nodes[i]) <= depth {
				planner.Nodes[i] = append(planner.Nodes[i], make(map[agentstate.State]*Node))
			}
			if node, exist := planner.Nodes[i][depth][state]; exist {
				nodes[i] = node
			} else {
				nodes[i] = planner.NodePool.Get().(*Node)
				planner.Nodes[i][depth][state] = nodes[i]
			}
			if nodes[i].RolloutCnt < planner.Config.ExpandThresh {
				nodes[i].RolloutCnt++
				nxtRollout[i] = true
			}
		}
		if nxtRollout[i] {
			actions[i] = Greedy(i, curStates, items, targetPos, planner.MapData, planner.RandGen)
		} else {
			// UCB アルゴリズムに従って行動選択
			validActions := GetValidActions(state, items[i], planner.MapData)
			actions[i] = nodes[i].Select(validActions)
		}
	}
	nxtStates, rewards, _ := agentstate.Next(curStates, actions, nxtRollout, items, planner.MapData, planner.Config, planner.RandGen, planner.NewItemProb)
	cumRewards := planner.update(turn+1, depth+1, nxtStates, items, nxtRollout, targetPos, iterIdx)
	for i := range curStates {
		cumRewards[i] = rewards[i] + planner.Config.DiscountFactor*cumRewards[i]
		if !rollout[i] {
			nodes[i].TotalCnt++
			nodes[i].SelectCnt[actions[i]]++
			nodes[i].CumReward[actions[i]] += cumRewards[i]
		}
	}
	return cumRewards
}

func (planner *Planner) Free() {
	for i := range planner.Nodes {
		for j := range planner.Nodes[i] {
			for _, node := range planner.Nodes[i][j] {
				node.Reset()
				planner.NodePool.Put(node)
			}
		}
	}
}
