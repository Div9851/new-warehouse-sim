package sim

import (
	"fmt"
	"math/rand"
	"sync"

	"github.com/Div9851/new-warehouse-sim/agentaction"
	"github.com/Div9851/new-warehouse-sim/agentstate"
	"github.com/Div9851/new-warehouse-sim/config"
	"github.com/Div9851/new-warehouse-sim/mapdata"
	"github.com/Div9851/new-warehouse-sim/mcts"
)

type Simulator struct {
	Turn        int
	States      agentstate.States
	Items       []map[mapdata.Pos]int
	LastActions agentaction.Actions
	ItemsCount  []int
	PickUpCount []int
	ClearCount  []int
	SubGoals    [][]mapdata.Pos
	Reserved    map[mapdata.Pos]int
	MapData     *mapdata.MapData
	SimRandGen  *rand.Rand
	RandGens    []*rand.Rand
	Verbose     bool
}

func New(mapData *mapdata.MapData, seed int64, verbose bool) *Simulator {
	simRandGen := rand.New(rand.NewSource(seed))
	randGens := []*rand.Rand{}
	states := agentstate.States{}
	items := []map[mapdata.Pos]int{}
	usedPos := make(map[mapdata.Pos]struct{})
	subGoals := make([][]mapdata.Pos, config.NumAgents)
	reserved := make(map[mapdata.Pos]int)
	for i := 0; i < config.NumAgents; i++ {
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
		randGen := rand.New(rand.NewSource(simRandGen.Int63()))
		randGens = append(randGens, randGen)
	}
	itemsCount := make([]int, config.NumAgents)
	pickUpCount := make([]int, config.NumAgents)
	clearCount := make([]int, config.NumAgents)
	return &Simulator{
		Turn:        0,
		States:      states,
		Items:       items,
		ItemsCount:  itemsCount,
		PickUpCount: pickUpCount,
		ClearCount:  clearCount,
		SubGoals:    subGoals,
		Reserved:    reserved,
		MapData:     mapData,
		SimRandGen:  simRandGen,
		RandGens:    randGens,
		Verbose:     verbose,
	}
}

func (sim *Simulator) Run() ([]int, []int, []int) {
	for {
		if sim.Verbose {
			sim.Dump()
		}
		if sim.Turn == config.LastTurn {
			break
		}
		// 行動決定フェーズ
		var wg sync.WaitGroup
		actions := make(agentaction.Actions, config.NumAgents)
		for id := 0; id < config.NumAgents; id++ {
			wg.Add(1)
			go func(id int) {
				planner := mcts.New(sim.MapData, sim.RandGens[id])
				items := make([]map[mapdata.Pos]int, config.NumAgents)
				for i := 0; i < config.NumAgents; i++ {
					if i == id {
						items[i] = sim.Items[i]
					} else {
						items[i] = make(map[mapdata.Pos]int)
					}
				}
				for i := 0; i < config.NumIters; i++ {
					planner.Update(sim.Turn, sim.States, id, items)
				}
				actions[id] = planner.BestAction(sim.States, id)
				wg.Done()
			}(id)
		}
		wg.Wait()
		// 行動フェーズ
		sim.Next(actions)
	}
	return sim.ItemsCount, sim.PickUpCount, sim.ClearCount
}

func (sim *Simulator) Next(actions agentaction.Actions) {
	sim.Turn++
	sim.LastActions = actions
	nxtStates, _, newItem := agentstate.Next(sim.States, actions, sim.Items, sim.MapData, sim.SimRandGen)
	sim.States = nxtStates
	for i := 0; i < config.NumAgents; i++ {
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
		if len(sim.SubGoals[i]) > 0 && sim.States[i].Pos == sim.SubGoals[i][0] {
			delete(sim.Reserved, sim.SubGoals[i][0])
			sim.SubGoals[i] = sim.SubGoals[i][1:]
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
		fmt.Printf("sub goals: %v\n", sim.SubGoals[i])
		fmt.Printf("items count: %d ", sim.ItemsCount[i])
		fmt.Printf("pickup count: %d ", sim.PickUpCount[i])
		fmt.Printf("clear count: %d\n", sim.ClearCount[i])
		fmt.Printf("has item: %v\n", state.HasItem)
	}
}
