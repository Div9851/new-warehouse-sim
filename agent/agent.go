package agent

import (
	"math/rand"

	"github.com/Div9851/new-warehouse-sim/config"
	"github.com/Div9851/new-warehouse-sim/mapdata"
)

type Agent struct {
	Id      int
	Pos     mapdata.Pos
	HasItem bool
}

type Agents []Agent

func (agents Agents) Next(actions Actions, items Items, mapData mapdata.MapData, allPos []mapdata.Pos, randGen *rand.Rand) (Agents, []float64, []bool, ItemsDiff) {
	var curPos []mapdata.Pos
	var hasItem []bool
	for _, agent := range agents {
		curPos = append(curPos, agent.Pos)
		hasItem = append(hasItem, agent.HasItem)
	}
	nxtPos, collision := NextPosMA(curPos, actions, mapData)
	n := len(agents)
	nxtAgents := make(Agents, n)
	rewards := make([]float64, n)
	newItem := make([]bool, n)
	itemsDiff := make(ItemsDiff, n)
	for i := range agents {
		if collision[i] {
			rewards[i] += config.Penalty
		}
	}
	for i := range agents {
		itemsDiff[i] = make(Item)
		switch actions[i] {
		case ACTION_PICKUP:
			if !hasItem[i] && items[i][curPos[i]] > 0 {
				hasItem[i] = true
				rewards[i] += config.Reward
				itemsDiff[i][curPos[i]]--
			}
		case ACTION_CLEAR:
			if r, c := curPos[i].R, curPos[i].C; hasItem[i] && mapData[r][c] == 'D' {
				hasItem[i] = false
				rewards[i] += config.Reward
			}
		}
		if randGen.Float64() < config.NewItemProb {
			newItemPos := allPos[randGen.Intn(len(allPos))]
			newItem[i] = true
			itemsDiff[i][newItemPos]++
		}
		nxtAgents[i] = Agent{Id: agents[i].Id, Pos: nxtPos[i], HasItem: hasItem[i]}
	}
	return nxtAgents, rewards, newItem, itemsDiff
}

func NextPosSA(curPos mapdata.Pos, action Action, mapData mapdata.MapData) mapdata.Pos {
	h, w := mapData.Size()
	nr, nc := curPos.R, curPos.C
	switch action {
	case ACTION_UP:
		nr--
	case ACTION_DOWN:
		nr++
	case ACTION_LEFT:
		nc--
	case ACTION_RIGHT:
		nc++
	}
	if 0 > nr || nr >= h || 0 > nc || nc >= w || mapData[nr][nc] == '#' {
		return curPos
	}
	return mapdata.Pos{R: nr, C: nc}
}

func NextPosMA(curPos []mapdata.Pos, actions Actions, mapData mapdata.MapData) ([]mapdata.Pos, []bool) {
	var nxtPos []mapdata.Pos
	posToId := make(map[mapdata.Pos]int)
	nxtCnt := make(map[mapdata.Pos]int)
	for i, cur := range curPos {
		nxt := NextPosSA(cur, actions[i], mapData)
		nxtPos = append(nxtPos, nxt)
		posToId[cur] = i
		nxtCnt[nxt]++
	}
	n := len(curPos)
	collision := make([]bool, n)
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
			collision[i] = false
			visited[i] = 2
			return
		}
		if _, ok := posToId[nxtPos[i]]; !ok {
			collision[i] = false
			visited[i] = 2
			return
		}
		j := posToId[nxtPos[i]]
		if visited[j] == 1 {
			collision[i] = true
			visited[i] = 2
			return
		}
		if visited[j] == 0 {
			dfs(j)
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
