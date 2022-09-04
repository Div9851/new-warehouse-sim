package mcts

import (
	"math"
	"math/rand"
	"sort"

	"github.com/Div9851/new-warehouse-sim/agentaction"
	"github.com/Div9851/new-warehouse-sim/agentstate"
	"github.com/Div9851/new-warehouse-sim/config"
	"github.com/Div9851/new-warehouse-sim/mapdata"
)

type ActionScore struct {
	Action agentaction.Action
	Score  float64
}

type ActionScores []ActionScore

func (a ActionScores) Len() int {
	return len(a)
}

func (a ActionScores) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

func (a ActionScores) Less(i, j int) bool {
	if a[i].Score != a[j].Score {
		return a[i].Score < a[j].Score
	}
	return a[i].Action < a[j].Action
}

type Node struct {
	CumReward  map[agentaction.Action]float64
	SelectCnt  map[agentaction.Action]float64
	TotalCnt   float64
	RolloutCnt int
	LastUpdate int
}

func (node *Node) Select(actions agentaction.Actions) agentaction.Action {
	actScores := ActionScores{}
	for _, action := range actions {
		var score float64
		if _, exist := node.SelectCnt[action]; exist {
			score = node.CumReward[action]/node.SelectCnt[action] + math.Sqrt(2*math.Log(node.TotalCnt)/node.SelectCnt[action])
		} else {
			score = math.Inf(1)
		}
		actScores = append(actScores, ActionScore{Action: action, Score: score})
	}
	sort.Sort(sort.Reverse(actScores))
	return actScores[0].Action
}

func (node *Node) BestAction() agentaction.Action {
	actScores := ActionScores{}
	for action := range node.CumReward {
		score := node.CumReward[action] / node.SelectCnt[action]
		actScores = append(actScores, ActionScore{Action: action, Score: score})
	}
	sort.Sort(sort.Reverse(actScores))
	return actScores[0].Action
}

type Planner struct {
	Nodes   [][]map[agentstate.State]*Node // [id][depth][pos]
	MapData *mapdata.MapData
	RandGen *rand.Rand
	IterIdx int
}

func New(mapData *mapdata.MapData, randGen *rand.Rand) *Planner {
	nodes := make([][]map[agentstate.State]*Node, config.NumAgents)
	return &Planner{Nodes: nodes, MapData: mapData, RandGen: randGen}
}

func (planner *Planner) BestAction(curStates agentstate.States, id int) agentaction.Action {
	state := curStates[id]
	node := planner.Nodes[id][0][state]
	return node.BestAction()
}

func (planner *Planner) Update(turn int, curStates agentstate.States, myId int, items []map[mapdata.Pos]int) {
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
	planner.update(turn, 0, curStates, myId, itemsCopy, rollout, targetPos)
	planner.IterIdx++
}

func (planner *Planner) update(turn int, depth int, curStates agentstate.States, myId int, items []map[mapdata.Pos]int, rollout []bool, targetPos []mapdata.Pos) []float64 {
	if turn == config.LastTurn || depth == config.MaxDepth {
		return make([]float64, config.NumAgents)
	}
	actions := make(agentaction.Actions, config.NumAgents)
	nxtRollout := make([]bool, config.NumAgents)
	copy(nxtRollout, rollout)
	for i, state := range curStates {
		if len(planner.Nodes[i]) <= depth {
			planner.Nodes[i] = append(planner.Nodes[i], make(map[agentstate.State]*Node))
		}
		if _, exist := planner.Nodes[i][depth][state]; !exist {
			planner.Nodes[i][depth][state] = &Node{
				CumReward:  make(map[agentaction.Action]float64),
				SelectCnt:  make(map[agentaction.Action]float64),
				TotalCnt:   0,
				RolloutCnt: 0,
				LastUpdate: 0,
			}
		}
		node := planner.Nodes[i][depth][state]
		if !rollout[i] && node.RolloutCnt < config.ExpandThresh {
			node.RolloutCnt++
			nxtRollout[i] = true
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
			actions[i] = node.Select(validActions)
		}
	}
	actionsCopy := make(agentaction.Actions, config.NumAgents)
	copy(actionsCopy, actions)
	nxtStates, rewards, _ := agentstate.Next(curStates, actions, items, planner.MapData, planner.RandGen)
	cumRewards := planner.update(turn+1, depth+1, nxtStates, myId, items, nxtRollout, targetPos)
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
