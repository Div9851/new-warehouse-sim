package agentstate

import (
	"math/rand"

	"github.com/Div9851/new-warehouse-sim/agentaction"
	"github.com/Div9851/new-warehouse-sim/config"
	"github.com/Div9851/new-warehouse-sim/mapdata"
)

type State struct {
	Pos     mapdata.Pos
	HasItem bool
}

type States []State

func Next(states States, actions agentaction.Actions, items []map[mapdata.Pos]int, subGoals [][]mapdata.Pos, mapData *mapdata.MapData, randGen *rand.Rand) (States, []float64, []bool) {
	var curPos []mapdata.Pos
	var hasItem []bool
	for _, state := range states {
		curPos = append(curPos, state.Pos)
		hasItem = append(hasItem, state.HasItem)
	}
	n := len(states)
	nxtStates := make(States, n)
	rewards := make([]float64, n)
	newItem := make([]bool, n)
	nxtPos, collision := NextPos(curPos, actions, mapData)
	for i := range states {
		if collision[i] {
			rewards[i] += config.CollisionPenalty
		}
		if len(subGoals[i]) > 0 && nxtPos[i] == subGoals[i][0] {
			rewards[i] += config.SubGoalReward
		}
		switch actions[i] {
		case agentaction.STAY:
			rewards[i] += config.StayPenalty
		case agentaction.PICKUP:
			if !hasItem[i] && items[i][curPos[i]] > 0 {
				hasItem[i] = true
				rewards[i] += config.PickupReward
				items[i][curPos[i]]--
				if items[i][curPos[i]] == 0 {
					delete(items[i], curPos[i])
				}
			}
		case agentaction.CLEAR:
			if hasItem[i] && curPos[i] == mapData.DepotPos {
				hasItem[i] = false
				rewards[i] += config.ClearReward
			}
		}
		if randGen.Float64() < config.NewItemProb {
			newItemPos := mapData.AllPos[randGen.Intn(len(mapData.AllPos))]
			newItem[i] = true
			items[i][newItemPos]++
		}
		nxtStates[i] = State{
			Pos:     nxtPos[i],
			HasItem: hasItem[i],
		}
	}
	return nxtStates, rewards, newItem
}

/*
func NextPosMA(curPos []mapdata.Pos, actions agentaction.Actions, mapData *mapdata.MapData) ([]mapdata.Pos, []bool) {
	n := len(curPos)
	var nxtPos []mapdata.Pos
	posToId := make(map[mapdata.Pos]int)
	nxtCnt := make(map[mapdata.Pos]int)
	for i, cur := range curPos {
		nxt := NextPosSA(cur, actions[i], mapData)
		nxtPos = append(nxtPos, nxt)
	}
	collision := make([]bool, n)
	for i, cur := range curPos {
		posToId[cur] = i
		nxtCnt[nxtPos[i]]++
	}
	visited := make([]int, n)
	var dfs func(int)
	dfs = func(i int) {
		visited[i] = 1
		if nxtCnt[nxtPos[i]] > 1 {
			collision[i] = true
			visited[i] = 2
			return
		}
		if nxtPos[i] == curPos[i] {
			visited[i] = 2
			return
		}
		if _, ok := posToId[nxtPos[i]]; !ok {
			visited[i] = 2
			return
		}
		j := posToId[nxtPos[i]]
		if visited[j] == 0 {
			dfs(j)
		} else if visited[j] == 1 {
			collision[i] = true
			visited[i] = 2
			return
		}
		collision[i] = collision[j]
		visited[i] = 2
	}
	for i := range nxtPos {
		if visited[i] == 0 {
			dfs(i)
		}
		if collision[i] {
			nxtPos[i] = curPos[i]
		}
	}
	return nxtPos, collision
}
*/

func NextPos(curPos []mapdata.Pos, actions agentaction.Actions, mapData *mapdata.MapData) ([]mapdata.Pos, []bool) {
	n := len(curPos)
	nxtPos := make([]mapdata.Pos, n)
	for i, cur := range curPos {
		nxtPos[i] = mapData.NextPos[cur.R][cur.C][actions[i]]
	}
	predId := make([]int, n)
	collision := make([]bool, n)
	visited := make([]int, n)
	for i := 0; i < n; i++ {
		predId[i] = -1
		for j := 0; j < n; j++ {
			if i == j {
				continue
			}
			if nxtPos[i] == curPos[j] {
				predId[i] = j
			}
			if nxtPos[i] == nxtPos[j] {
				collision[i] = true
				visited[i] = 2
			}
		}
	}
	var dfs func(int)
	dfs = func(i int) {
		visited[i] = 1
		if predId[i] == -1 {
			visited[i] = 2
			return
		}
		j := predId[i]
		if visited[j] == 0 {
			dfs(j)
		} else if visited[j] == 1 {
			collision[i] = true
			visited[i] = 2
			return
		}
		collision[i] = collision[j]
		visited[i] = 2
	}
	for i := 0; i < n; i++ {
		if visited[i] == 0 {
			dfs(i)
		}
		if collision[i] {
			nxtPos[i] = curPos[i]
		}
	}
	return nxtPos, collision
}
