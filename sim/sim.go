package sim

import (
	"fmt"
	"math/rand"
	"sort"
	"sync"

	"github.com/Div9851/new-warehouse-sim/agentaction"
	"github.com/Div9851/new-warehouse-sim/agentstate"
	"github.com/Div9851/new-warehouse-sim/config"
	"github.com/Div9851/new-warehouse-sim/fduct"
	"github.com/Div9851/new-warehouse-sim/mapdata"
)

type Bid struct {
	Id    int
	Route []mapdata.Pos
	Value float64
}

func GetRoute(curState agentstate.State, items map[mapdata.Pos]int, reserved map[mapdata.Pos]struct{}, mapData *mapdata.MapData) []mapdata.Pos {
	que := []mapdata.Pos{curState.Pos}
	vis := make(map[mapdata.Pos]struct{})
	vis[curState.Pos] = struct{}{}
	prev := make(map[mapdata.Pos]mapdata.Pos)
	goal := mapdata.NonePos
	for len(que) > 0 {
		curPos := que[0]
		que = que[1:]
		if !curState.HasItem && items[curPos] > 0 {
			goal = curPos
			break
		}
		if curState.HasItem && curPos == mapData.DepotPos {
			goal = curPos
			break
		}
		actions := mapData.ValidActions[curPos.R][curPos.C]
		for _, action := range actions {
			nxtPos := mapData.NextPos[curPos.R][curPos.C][action]
			if _, exist := vis[nxtPos]; exist {
				continue
			}
			if _, exist := reserved[nxtPos]; exist {
				continue
			}
			que = append(que, nxtPos)
			vis[nxtPos] = struct{}{}
			prev[nxtPos] = curPos
		}
	}
	if goal == mapdata.NonePos {
		return nil
	}
	var route []mapdata.Pos
	for goal != curState.Pos {
		route = append(route, goal)
		goal = prev[goal]
	}
	for i, j := 0, len(route)-1; i < j; i, j = i+1, j-1 {
		route[i], route[j] = route[j], route[i]
	}
	return route
}

type Simulator struct {
	Turn            int
	States          agentstate.States
	Items           []map[mapdata.Pos]int
	Budgets         []float64
	LastActions     agentaction.Actions
	ItemsCount      []int
	PickUpCount     []int
	ClearCount      []int
	Routes          [][]mapdata.Pos
	MapData         *mapdata.MapData
	SimRandGen      *rand.Rand
	PlannerRandGens []*rand.Rand
	Verbose         bool
}

func New(mapData *mapdata.MapData, seed int64, verbose bool) *Simulator {
	simRandGen := rand.New(rand.NewSource(seed))
	plannerRandGens := []*rand.Rand{}
	states := agentstate.States{}
	items := []map[mapdata.Pos]int{}
	budgets := []float64{}
	usedPos := make(map[mapdata.Pos]struct{})
	routes := make([][]mapdata.Pos, config.NumAgents)
	for i := 0; i < config.NumAgents; i++ {
		plannerRandGens = append(plannerRandGens, rand.New(rand.NewSource(simRandGen.Int63())))
		var startPos mapdata.Pos
		for {
			startPos = mapData.AllPos[simRandGen.Intn(len(mapData.AllPos))]
			if _, isUsed := usedPos[startPos]; !isUsed {
				break
			}
		}
		usedPos[startPos] = struct{}{}
		newState := agentstate.State{
			Pos:     startPos,
			HasItem: false,
		}
		states = append(states, newState)
		items = append(items, make(map[mapdata.Pos]int))
		budgets = append(budgets, config.InitialBudget)
	}
	itemsCount := make([]int, config.NumAgents)
	pickUpCount := make([]int, config.NumAgents)
	clearCount := make([]int, config.NumAgents)
	return &Simulator{
		Turn:            0,
		States:          states,
		Items:           items,
		Budgets:         budgets,
		ItemsCount:      itemsCount,
		PickUpCount:     pickUpCount,
		ClearCount:      clearCount,
		Routes:          routes,
		MapData:         mapData,
		SimRandGen:      simRandGen,
		PlannerRandGens: plannerRandGens,
		Verbose:         verbose,
	}
}

func (sim *Simulator) Run() ([]int, []int, []int) {
	var nodePool = &sync.Pool{
		New: func() interface{} {
			return fduct.NewNode()
		},
	}
	for {
		if sim.Verbose {
			sim.Dump()
		}
		if sim.Turn == config.LastTurn {
			break
		}
		var wg sync.WaitGroup
		planners := make([]*fduct.Planner, config.NumAgents)
		actions := make(agentaction.Actions, config.NumAgents)
		plannedRoute := make([][]mapdata.Pos, config.NumAgents)
		// プランニングフェーズ
		for id := 0; id < config.NumAgents; id++ {
			wg.Add(1)
			planners[id] = fduct.New(sim.MapData, sim.PlannerRandGens[id], nodePool, 0)
			go func(id int) {
				for iter := 0; iter < config.NumIters; iter++ {
					planners[id].Update(sim.Turn, sim.States, sim.Items, sim.Routes, iter)
				}
				actions[id] = planners[id].GetBestAction(id, sim.States[id], sim.Items[id])
				plannedRoute[id] = planners[id].GetRoute(id, sim.States[id], sim.Items[id])
				planners[id].Free()
				wg.Done()
			}(id)
		}
		wg.Wait()
		// 予約フェーズ
		reserved := make(map[mapdata.Pos]struct{})
		for id := 0; id < config.NumAgents; id++ {
			for _, pos := range sim.Routes[id] {
				reserved[pos] = struct{}{}
			}
		}
		var bids []Bid
		for id := 0; id < config.NumAgents; id++ {
			if len(sim.Routes[id]) > 0 {
				continue
			}
			route := GetRoute(sim.States[id], sim.Items[id], reserved, sim.MapData)
			if len(route) == 0 {
				continue
			}
			var r float64
			if len(plannedRoute[id]) > 0 {
				r = float64(len(route)) / float64(len(plannedRoute[id]))
			} else {
				r = 1
			}
			if r <= 1 {
				bids = append(bids, Bid{Id: id, Route: route, Value: sim.Budgets[id] * r})
			}
		}
		sort.Slice(bids, func(i, j int) bool { return bids[i].Value > bids[j].Value })
		for _, bid := range bids {
			skip := false
			for _, pos := range bid.Route {
				if _, exist := reserved[pos]; exist {
					skip = true
					break
				}
			}
			if skip {
				continue
			}
			for _, pos := range bid.Route {
				reserved[pos] = struct{}{}
			}
			sim.Routes[bid.Id] = bid.Route
			sim.Budgets[bid.Id] -= bid.Value
		}
		sim.Next(actions)
	}
	return sim.ItemsCount, sim.PickUpCount, sim.ClearCount
}

func (sim *Simulator) Next(actions agentaction.Actions) {
	sim.Turn++
	sim.LastActions = actions
	free := make([]bool, config.NumAgents)
	nxtStates, _, newItem := agentstate.Next(sim.States, actions, free, sim.Items, sim.Routes, sim.MapData, sim.SimRandGen, config.NewItemProb)
	sim.States = nxtStates
	for i := 0; i < config.NumAgents; i++ {
		if newItem[i] {
			sim.ItemsCount[i]++
			sim.Budgets[i] += config.IncreaseBudget
		}
		// PICKUP や CLEAR は可能なときにしか選ばないと仮定
		if actions[i] == agentaction.PICKUP {
			sim.PickUpCount[i]++
			for len(sim.Routes[i]) > 0 {
				sim.Routes[i] = sim.Routes[i][1:]
			}
		}
		if actions[i] == agentaction.CLEAR {
			sim.ClearCount[i]++
			for len(sim.Routes[i]) > 0 {
				sim.Routes[i] = sim.Routes[i][1:]
			}
		}
		if len(sim.Routes[i]) > 0 && sim.States[i].Pos == sim.Routes[i][0] {
			sim.Routes[i] = sim.Routes[i][1:]
		}
	}
}

func (sim *Simulator) Dump() {
	fmt.Printf("TURN %d:\n", sim.Turn)
	mapData := [][]byte{}
	for _, row := range sim.MapData.Text {
		mapData = append(mapData, []byte(row))
	}
	for i, agent := range sim.States {
		mapData[agent.Pos.R][agent.Pos.C] = byte('0' + i)
	}
	for _, row := range mapData {
		fmt.Println(string(row))
	}
	fmt.Println("[ITEMS]")
	fmt.Printf("%v\n", sim.Items)
	for i, state := range sim.States {
		fmt.Printf("[AGENT %d]\n", i)
		if len(sim.LastActions) > 0 {
			fmt.Printf("last action: %s\n", sim.LastActions[i].ToStr())
		}
		fmt.Printf("pos: %v\n", state.Pos)
		fmt.Printf("budget: %f\n", sim.Budgets[i])
		fmt.Printf("route: %v\n", sim.Routes[i])
		fmt.Printf("items count: %d ", sim.ItemsCount[i])
		fmt.Printf("pickup count: %d ", sim.PickUpCount[i])
		fmt.Printf("clear count: %d\n", sim.ClearCount[i])
		fmt.Printf("has item: %v\n", state.HasItem)
	}
}
