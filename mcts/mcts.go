package mcts

import (
	"math"
	"math/rand"

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

type Planner struct {
	Nodes   [][]map[agentstate.State]*Node // [id][depth][state]
	Id      int
	MapData *mapdata.MapData
	RandGen *rand.Rand
	IterIdx int
}

func New(id int, mapData *mapdata.MapData, randGen *rand.Rand) *Planner {
	nodes := make([][]map[agentstate.State]*Node, config.NumAgents)
	return &Planner{
		Nodes:   nodes,
		Id:      id,
		MapData: mapData,
		RandGen: randGen,
		IterIdx: 0,
	}
}

func (planner *Planner) BestAction(curStates agentstate.States) agentaction.Action {
	state := curStates[planner.Id]
	node := planner.Nodes[planner.Id][0][state]
	return node.BestAction()
}

func (planner *Planner) Update(turn int, curStates agentstate.States, items []map[mapdata.Pos]int) {
	rollout := make([]bool, config.NumAgents)
	targetPos := []mapdata.Pos{}
	itemsCopy := []map[mapdata.Pos]int{}
	for i := 0; i < config.NumAgents; i++ {
		targetPos = append(targetPos, mapdata.NonePos)
		m := make(map[mapdata.Pos]int)
		for pos, itemNum := range items[i] {
			m[pos] = itemNum
		}
		itemsCopy = append(itemsCopy, m)
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
	for i, state := range curStates {
		if !rollout[i] {
			if len(planner.Nodes[i]) <= depth {
				planner.Nodes[i] = append(planner.Nodes[i], make(map[agentstate.State]*Node))
			}
			if _, exist := planner.Nodes[i][depth][state]; !exist {
				planner.Nodes[i][depth][state] = NewNode()
			}
			node := planner.Nodes[i][depth][state]
			if node.RolloutCnt < config.ExpandThresh {
				node.RolloutCnt++
				nxtRollout[i] = true
			}
		}
		validActions := make(agentaction.Actions, len(planner.MapData.ValidActions[state.Pos]))
		copy(validActions, planner.MapData.ValidActions[state.Pos])
		if !state.HasItem && items[i][state.Pos] > 0 {
			validActions = append(validActions, agentaction.PICKUP)
		} else if state.HasItem && state.Pos == planner.MapData.DepotPos {
			validActions = append(validActions, agentaction.CLEAR)
		} else {
			validActions = append(validActions, agentaction.STAY)
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
						if itemNum > 0 && d > minDist[state.Pos][pos] {
							d = minDist[state.Pos][pos]
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
					nxtPos := agentstate.NextPosSA(state.Pos, action, planner.MapData)
					if minDist[state.Pos][targetPos[i]] > minDist[nxtPos][targetPos[i]] {
						optimal = append(optimal, action)
					}
				}
				actions[i] = optimal[planner.RandGen.Intn(len(optimal))]
			}
		} else {
			// UCB アルゴリズムに従って行動選択
			node := planner.Nodes[i][depth][state]
			actions[i] = node.Select(validActions)
		}
	}
	actionsCopy := make(agentaction.Actions, config.NumAgents)
	copy(actionsCopy, actions)
	nxtStates, rewards, _ := agentstate.Next(curStates, actions, items, planner.MapData, planner.RandGen)
	cumRewards := planner.update(turn+1, depth+1, nxtStates, items, nxtRollout, targetPos)
	for i, state := range curStates {
		cumRewards[i] = rewards[i] + config.DiscountFactor*cumRewards[i]
		if !rollout[i] {
			node := planner.Nodes[i][depth][state]
			decay := math.Pow(config.DecayRate, float64(planner.IterIdx-node.LastUpdate))
			node.TotalCnt *= decay
			for action := range node.SelectCnt {
				node.SelectCnt[action] *= decay
				node.CumReward[action] *= decay
			}
			node.TotalCnt++
			node.SelectCnt[actions[i]]++
			node.CumReward[actions[i]] += cumRewards[i]
			node.LastUpdate = planner.IterIdx
		}
	}
	return cumRewards
}
