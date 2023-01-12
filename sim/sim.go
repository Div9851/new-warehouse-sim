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

type Request struct {
	From int
	Pos  mapdata.Pos
}

var dummyRequest = Request{
	From: -1,
	Pos:  mapdata.NonePos,
}

type Simulator struct {
	Turn            int
	States          agentstate.States
	Items           []map[mapdata.Pos]int
	LastActions     agentaction.Actions
	ItemsCount      []int
	PickUpCount     []int
	ClearCount      []int
	MapData         *mapdata.MapData
	SimRandGen      *rand.Rand
	PlannerRandGens []*rand.Rand
	Config          *config.Config
	Verbose         bool
}

func New(mapData *mapdata.MapData, config *config.Config, verbose bool, seed int64) *Simulator {
	simRandGen := rand.New(rand.NewSource(seed))
	plannerRandGens := []*rand.Rand{}
	states := agentstate.States{}
	items := []map[mapdata.Pos]int{}
	usedPos := make(map[mapdata.Pos]struct{})
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
	}
	itemsCount := make([]int, config.NumAgents)
	pickUpCount := make([]int, config.NumAgents)
	clearCount := make([]int, config.NumAgents)
	return &Simulator{
		Turn:            0,
		States:          states,
		Items:           items,
		ItemsCount:      itemsCount,
		PickUpCount:     pickUpCount,
		ClearCount:      clearCount,
		MapData:         mapData,
		SimRandGen:      simRandGen,
		PlannerRandGens: plannerRandGens,
		Config:          config,
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
		if sim.Turn == sim.Config.LastTurn {
			break
		}
		depotPos := sim.MapData.DepotPos
		minDist := sim.MapData.MinDist
		// 負荷分散
		if sim.Config.EnableLoadBalance {
			load := make([]float64, sim.Config.NumAgents)
			avgLoad := 0.0
			for id := 0; id < sim.Config.NumAgents; id++ {
				if sim.States[id].HasItem {
					pos := sim.States[id].Pos
					load[id] += float64(minDist[depotPos.R][depotPos.C][pos.R][pos.C])
				}
				for pos, cnt := range sim.Items[id] {
					load[id] += float64(minDist[depotPos.R][depotPos.C][pos.R][pos.C] * cnt)
				}
				avgLoad += load[id]
			}
			avgLoad /= float64(sim.Config.NumAgents)
			var requests []Request
			acceptIds := make(map[Request][]int)
			for id := 0; id < sim.Config.NumAgents; id++ {
				if load[id] > avgLoad {
					limit := load[id] - avgLoad
					cands := []mapdata.Pos{}
					for pos := range sim.Items[id] {
						dist := float64(minDist[depotPos.R][depotPos.C][pos.R][pos.C])
						if dist <= limit {
							cands = append(cands, pos)
						}
					}
					switch sim.Config.OfferStrategy {
					case "NEAREST_FROM_DEPOT":
						sort.Slice(cands, func(i, j int) bool {
							d1 := minDist[depotPos.R][depotPos.C][cands[i].R][cands[i].C]
							d2 := minDist[depotPos.R][depotPos.C][cands[j].R][cands[j].C]
							return d1 < d2
						})
					case "FARTHEST_FROM_DEPOT":
						sort.Slice(cands, func(i, j int) bool {
							d1 := minDist[depotPos.R][depotPos.C][cands[i].R][cands[i].C]
							d2 := minDist[depotPos.R][depotPos.C][cands[j].R][cands[j].C]
							return d1 > d2
						})
					}
					if len(cands) > 0 {
						requests = append(requests, Request{
							From: id,
							Pos:  cands[0],
						})
					}
				}
			}
			for id := 0; id < sim.Config.NumAgents; id++ {
				if load[id] < avgLoad {
					limit := avgLoad - load[id]
					cands := []Request{}
					for _, req := range requests {
						dist := float64(minDist[depotPos.R][depotPos.C][req.Pos.R][req.Pos.C])
						if dist <= limit {
							cands = append(cands, req)
						}
					}
					switch sim.Config.AcceptStrategy {
					case "NEAREST_FROM_DEPOT":
						sort.Slice(cands, func(i, j int) bool {
							d1 := minDist[depotPos.R][depotPos.C][cands[i].Pos.R][cands[i].Pos.C]
							d2 := minDist[depotPos.R][depotPos.C][cands[j].Pos.R][cands[j].Pos.C]
							return d1 < d2
						})
					case "FARTHEST_FROM_DEPOT":
						sort.Slice(cands, func(i, j int) bool {
							d1 := minDist[depotPos.R][depotPos.C][cands[i].Pos.R][cands[i].Pos.C]
							d2 := minDist[depotPos.R][depotPos.C][cands[j].Pos.R][cands[j].Pos.C]
							return d1 > d2
						})
					}
					if len(cands) > 0 {
						acceptIds[cands[0]] = append(acceptIds[cands[0]], id)
					}
				}
			}
			for _, req := range requests {
				cands := acceptIds[req]
				// NEAREST_FROM_ITEM only
				sort.Slice(cands, func(i, j int) bool {
					pos1 := sim.States[cands[i]].Pos
					pos2 := sim.States[cands[j]].Pos
					d1 := minDist[req.Pos.R][req.Pos.C][pos1.R][pos1.C]
					d2 := minDist[req.Pos.R][req.Pos.C][pos2.R][pos2.C]
					return d1 < d2
				})
				if len(cands) > 0 {
					// req.From --> cands[0]
					from := req.From
					to := cands[0]
					sim.ItemsCount[from]--
					sim.ItemsCount[to]++
					sim.Items[from][req.Pos]--
					if sim.Items[from][req.Pos] == 0 {
						delete(sim.Items[from], req.Pos)
					}
					sim.Items[to][req.Pos]++
				}
			}
		}
		// プランニングフェーズ
		planners := make([]*fduct.Planner, sim.Config.NumAgents)
		actions := make(agentaction.Actions, sim.Config.NumAgents)
		var wg sync.WaitGroup
		for id := 0; id < sim.Config.NumAgents; id++ {
			wg.Add(1)
			planners[id] = fduct.New(sim.MapData, sim.Config, sim.PlannerRandGens[id], nodePool, 0)
			go func(id int) {
				for iter := 0; iter < sim.Config.NumIters; iter++ {
					planners[id].Update(sim.Turn, sim.States, sim.Items, iter)
				}
				actions[id], _ = planners[id].GetBestAction(id, sim.States[id], sim.Items[id])
				planners[id].Free()
				wg.Done()
			}(id)
		}
		wg.Wait()
		sim.Next(actions)
	}
	return sim.ItemsCount, sim.PickUpCount, sim.ClearCount
}

func (sim *Simulator) Next(actions agentaction.Actions) {
	sim.Turn++
	sim.LastActions = actions
	free := make([]bool, sim.Config.NumAgents)
	nxtStates, _, newItem := agentstate.Next(sim.States, actions, free, sim.Items, sim.MapData, sim.Config, sim.SimRandGen, sim.Config.NewItemProb)
	sim.States = nxtStates
	for i := 0; i < sim.Config.NumAgents; i++ {
		if newItem[i] {
			sim.ItemsCount[i]++
		}
		// PICKUP や CLEAR は可能なときにしか選ばないと仮定
		if actions[i] == agentaction.PICKUP {
			sim.PickUpCount[i]++
		}
		if actions[i] == agentaction.CLEAR {
			sim.ClearCount[i]++
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
		fmt.Printf("items count: %d ", sim.ItemsCount[i])
		fmt.Printf("pickup count: %d ", sim.PickUpCount[i])
		fmt.Printf("clear count: %d\n", sim.ClearCount[i])
	}
}
