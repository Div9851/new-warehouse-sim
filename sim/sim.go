package sim

import (
	"fmt"
	"math"
	"math/rand"
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

type Simulator struct {
	Turn            int
	States          agentstate.States
	Items           []map[mapdata.Pos]int
	Budget          []float64
	Priority        []float64
	LastActions     agentaction.Actions
	ItemsCount      []int
	PickUpCount     []int
	ClearCount      []int
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
	budget := []float64{}
	priority := make([]float64, config.NumAgents)
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
		budget = append(budget, config.InitialBudget)
	}
	itemsCount := make([]int, config.NumAgents)
	pickUpCount := make([]int, config.NumAgents)
	clearCount := make([]int, config.NumAgents)
	return &Simulator{
		Turn:            0,
		States:          states,
		Items:           items,
		Budget:          budget,
		Priority:        priority,
		ItemsCount:      itemsCount,
		PickUpCount:     pickUpCount,
		ClearCount:      clearCount,
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
		if sim.Turn > 0 && sim.Turn%config.BiddingInterval == 0 {
			// 入札フェーズ
			worstPlanners := make([]*fduct.Planner, config.NumAgents)
			worstRewards := make([]float64, config.NumAgents)
			for id := 0; id < config.NumAgents; id++ {
				wg.Add(1)
				worstPlanners[id] = fduct.New(sim.MapData, sim.PlannerRandGens[id], nodePool, 0)
				go func(id int) {
					priority := make([]float64, config.NumAgents)
					priority[id] = math.Inf(-1)
					for iter := 0; iter < config.NumIters; iter++ {
						worstPlanners[id].Update(sim.Turn, sim.States, sim.Items, priority, iter)
					}
					_, worstRewards[id] = worstPlanners[id].GetBestAction(id, sim.States[id], sim.Items[id])
					wg.Done()
				}(id)
			}
			wg.Wait()
			bestPlanners := make([]*fduct.Planner, config.NumAgents)
			bestRewards := make([]float64, config.NumAgents)
			for id := 0; id < config.NumAgents; id++ {
				wg.Add(1)
				bestPlanners[id] = fduct.New(sim.MapData, sim.PlannerRandGens[id], nodePool, 0)
				go func(id int) {
					priority := make([]float64, config.NumAgents)
					priority[id] = math.Inf(0)
					for iter := 0; iter < config.NumIters; iter++ {
						bestPlanners[id].Update(sim.Turn, sim.States, sim.Items, priority, iter)
					}
					_, bestRewards[id] = bestPlanners[id].GetBestAction(id, sim.States[id], sim.Items[id])
					wg.Done()
				}(id)
			}
			wg.Wait()
			for id := 0; id < config.NumAgents; id++ {
				if worstRewards[id] >= bestRewards[id] {
					sim.Priority[id] = 0
					continue
				}
				r := math.Min((bestRewards[id]-worstRewards[id])/math.Abs(worstRewards[id]), 1)
				sim.Priority[id] = sim.Budget[id] * r
				sim.Budget[id] -= sim.Priority[id]
			}
		}
		// プランニングフェーズ
		planners := make([]*fduct.Planner, config.NumAgents)
		actions := make(agentaction.Actions, config.NumAgents)
		for id := 0; id < config.NumAgents; id++ {
			wg.Add(1)
			planners[id] = fduct.New(sim.MapData, sim.PlannerRandGens[id], nodePool, 0)
			go func(id int) {
				for iter := 0; iter < config.NumIters; iter++ {
					planners[id].Update(sim.Turn, sim.States, sim.Items, sim.Priority, iter)
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
	free := make([]bool, config.NumAgents)
	nxtStates, _, newItem := agentstate.Next(sim.States, actions, free, sim.Items, sim.Priority, sim.MapData, sim.SimRandGen, config.NewItemProb)
	sim.States = nxtStates
	for i := 0; i < config.NumAgents; i++ {
		if newItem[i] {
			sim.ItemsCount[i]++
			sim.Budget[i] += config.IncreaseBudget
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
		fmt.Printf("budget: %f\n", sim.Budget[i])
		fmt.Printf("priority: %f\n", sim.Priority[i])
		fmt.Printf("items count: %d ", sim.ItemsCount[i])
		fmt.Printf("pickup count: %d ", sim.PickUpCount[i])
		fmt.Printf("clear count: %d\n", sim.ClearCount[i])
		fmt.Printf("has item: %v\n", state.HasItem)
	}
}
