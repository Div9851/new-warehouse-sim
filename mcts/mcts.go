package mcts

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
	LastUpdate int
}

func NewNode() *Node {
	return &Node{
		CumReward:  make([]float64, agentaction.COUNT),
		SelectCnt:  make([]float64, agentaction.COUNT),
		TotalCnt:   0,
		RolloutCnt: 0,
		LastUpdate: 0,
	}
}

func (node *Node) UCB1(action agentaction.Action) float64 {
	if node.SelectCnt[action] == 0 {
		return math.Inf(1)
	}
	score := node.CumReward[action]/node.SelectCnt[action] + math.Sqrt(2*math.Log(node.TotalCnt)/node.SelectCnt[action])
	return score
}

/*
func (node *Node) UCB1Tuned(action agentaction.Action) float64 {
	if node.SelectCnt[action] == 0 {
		return math.Inf(1)
	}
	// 分散 = （二乗の平均）-（平均の二乗）
	v := node.CumRewardSquared[action]/node.SelectCnt[action] - math.Pow(node.CumReward[action]/node.SelectCnt[action], 2)
	c := math.Min(0.25, v+math.Sqrt(2*math.Log(node.TotalCnt)/node.SelectCnt[action]))
	score := node.CumReward[action]/node.SelectCnt[action] + math.Sqrt(c*math.Log(node.TotalCnt)/node.SelectCnt[action])
	return score
}
*/

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

func (node *Node) BestAction() agentaction.Action {
	selected := -1
	maxScore := math.Inf(-1)
	for action := range node.SelectCnt {
		if maxScore < node.SelectCnt[action] {
			maxScore = node.SelectCnt[action]
			selected = action
		}
	}
	return agentaction.Action(selected)
}

func (node *Node) Reset() {
	for i := range node.CumReward {
		node.CumReward[i] = 0
		node.SelectCnt[i] = 0
	}
	node.TotalCnt = 0
	node.RolloutCnt = 0
	node.LastUpdate = 0
}

type Planner struct {
	Nodes    [][]map[agentstate.State]*Node // [id][depth][state]
	MapData  *mapdata.MapData
	RandGen  *rand.Rand
	NodePool *sync.Pool
	IterIdx  int
}

func New(mapData *mapdata.MapData, randGen *rand.Rand, nodePool *sync.Pool) *Planner {
	nodes := make([][]map[agentstate.State]*Node, config.NumAgents)
	return &Planner{
		Nodes:    nodes,
		MapData:  mapData,
		RandGen:  randGen,
		NodePool: nodePool,
		IterIdx:  0,
	}
}

func (planner *Planner) BestActions(curStates agentstate.States) agentaction.Actions {
	actions := make(agentaction.Actions, config.NumAgents)
	for i, state := range curStates {
		node := planner.Nodes[i][0][state]
		actions[i] = node.BestAction()
	}
	return actions
}

func (planner *Planner) Update(turn int, curStates agentstate.States, items []map[mapdata.Pos]int) {
	rollout := make([]bool, config.NumAgents)
	targetPos := make([]mapdata.Pos, config.NumAgents)
	itemsCopy := make([]map[mapdata.Pos]int, config.NumAgents)
	for i := 0; i < config.NumAgents; i++ {
		targetPos[i] = mapdata.NonePos
		itemsCopy[i] = make(map[mapdata.Pos]int)
		for pos, itemNum := range items[i] {
			itemsCopy[i][pos] = itemNum
		}
	}
	planner.update(turn, 0, curStates, itemsCopy, rollout, targetPos)
	planner.IterIdx++
}

func (planner *Planner) update(turn int, depth int, curStates agentstate.States, items []map[mapdata.Pos]int, rollout []bool, targetPos []mapdata.Pos) []float64 {
	if turn == config.LastTurn || depth == config.MaxDepth {
		return make([]float64, config.NumAgents)
	}
	actions := make(agentaction.Actions, config.NumAgents)
	nxtRollout := make([]bool, config.NumAgents)
	copy(nxtRollout, rollout)
	nodes := make([]*Node, config.NumAgents)
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
			if nodes[i].RolloutCnt < config.ExpandThresh {
				nodes[i].RolloutCnt++
				nxtRollout[i] = true
			}
		}
		validActions := make(agentaction.Actions, len(planner.MapData.ValidActions[state.Pos.R][state.Pos.C]))
		copy(validActions, planner.MapData.ValidActions[state.Pos.R][state.Pos.C])
		if !state.HasItem && items[i][state.Pos] > 0 {
			validActions = append(validActions, agentaction.PICKUP)
		}
		if state.HasItem && state.Pos == planner.MapData.DepotPos {
			validActions = append(validActions, agentaction.CLEAR)
		}
		minDist := planner.MapData.MinDist
		if nxtRollout[i] {
			// ロールアウトポリシーに従って行動選択
			if targetPos[i] == mapdata.NonePos {
				if state.HasItem {
					// アイテムをもっているなら、デポを目的地にする
					targetPos[i] = planner.MapData.DepotPos
				} else {
					// アイテムをもっていないなら、アイテムのある頂点のうち最も近い頂点を目的地にする
					d := math.MaxInt
					for pos, itemNum := range items[i] {
						if itemNum > 0 && d > minDist[state.Pos.R][state.Pos.C][pos.R][pos.C] {
							d = minDist[state.Pos.R][state.Pos.C][pos.R][pos.C]
							targetPos[i] = pos
						}
					}
					// アイテムのある頂点がない場合、ランダムに行動
					if d == math.MaxInt {
						actions[i] = validActions[planner.RandGen.Intn(len(validActions))]
						continue
					}
				}
			}
			if state.Pos == targetPos[i] {
				if state.HasItem {
					actions[i] = agentaction.CLEAR
				} else {
					actions[i] = agentaction.PICKUP
				}
				targetPos[i] = mapdata.NonePos
			} else {
				optimal := agentaction.Actions{}
				for _, action := range validActions {
					nxtPos := planner.MapData.NextPos[state.Pos.R][state.Pos.C][action]
					if minDist[state.Pos.R][state.Pos.C][targetPos[i].R][targetPos[i].C] > minDist[nxtPos.R][nxtPos.C][targetPos[i].R][targetPos[i].C] {
						optimal = append(optimal, action)
					}
				}
				actions[i] = optimal[planner.RandGen.Intn(len(optimal))]
			}
		} else {
			// UCB アルゴリズムに従って行動選択
			actions[i] = nodes[i].Select(validActions)
		}
	}
	nxtStates, rewards, _ := agentstate.Next(curStates, actions, items, planner.MapData, planner.RandGen)
	cumRewards := planner.update(turn+1, depth+1, nxtStates, items, nxtRollout, targetPos)
	for i := 0; i < config.NumAgents; i++ {
		cumRewards[i] = rewards[i] + config.DiscountFactor*cumRewards[i]
		if !rollout[i] {
			decay := math.Pow(config.DecayRate, float64(planner.IterIdx-nodes[i].LastUpdate))
			nodes[i].TotalCnt *= decay
			for action := range nodes[i].SelectCnt {
				nodes[i].SelectCnt[action] *= decay
				nodes[i].CumReward[action] *= decay
			}
			nodes[i].TotalCnt++
			nodes[i].SelectCnt[actions[i]]++
			nodes[i].CumReward[actions[i]] += cumRewards[i]
			nodes[i].LastUpdate = planner.IterIdx
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
