package sim

import (
	"fmt"
	"math/rand"

	"github.com/Div9851/new-warehouse-sim/agent"
	"github.com/Div9851/new-warehouse-sim/mapdata"
)

type Simulator struct {
	Turn        int
	Agents      agent.Agents
	Items       agent.Items
	LastActions agent.Actions
	ItemsCount  []int
	PickUpCount []int
	ClearCount  []int
	Reserved    map[mapdata.PosTurn]int
	MapData     mapdata.MapData
	AllPos      []mapdata.Pos
	SimRandGen  *rand.Rand
	RandGens    []*rand.Rand
}

func New(numAgents int, mapData mapdata.MapData, seed int64) *Simulator {
	allPos := mapData.GetAllPos()
	simRandGen := rand.New(rand.NewSource(seed))
	randGens := []*rand.Rand{}
	agents := []agent.Agent{}
	items := agent.Items{}
	usedPos := make(map[mapdata.Pos]struct{})
	for i := 0; i < numAgents; i++ {
		var startPos mapdata.Pos
		for {
			startPos = allPos[simRandGen.Intn(len(allPos))]
			if _, ok := usedPos[startPos]; !ok {
				break
			}
		}
		usedPos[startPos] = struct{}{}
		newAgent := agent.Agent{Id: i, Pos: startPos, HasItem: false}
		agents = append(agents, newAgent)
		items = append(items, make(agent.Item))
		randGen := rand.New(rand.NewSource(simRandGen.Int63()))
		randGens = append(randGens, randGen)
	}
	itemsCount := make([]int, numAgents)
	pickUpCount := make([]int, numAgents)
	clearCount := make([]int, numAgents)
	reserved := make(map[mapdata.PosTurn]int)
	return &Simulator{
		Turn:        0,
		Agents:      agents,
		Items:       items,
		ItemsCount:  itemsCount,
		PickUpCount: pickUpCount,
		ClearCount:  clearCount,
		Reserved:    reserved,
		MapData:     mapData,
		AllPos:      allPos,
		SimRandGen:  simRandGen,
		RandGens:    randGens,
	}
}

func (sim *Simulator) Next(actions agent.Actions) {
	sim.Turn++
	sim.LastActions = actions
	nxtAgents, rewards, newItem, itemsDiff := sim.Agents.Next(actions, sim.Items, sim.MapData, sim.AllPos, sim.SimRandGen)
	for i := range itemsDiff {
		for k, v := range itemsDiff[i] {
			sim.Items[i][k] += v
			if sim.Items[i][k] == 0 {
				delete(sim.Items[i], k)
			}
		}
	}
	sim.Agents = nxtAgents
	numAgents := len(sim.Agents)
	for i := 0; i < numAgents; i++ {
		if newItem[i] {
			sim.ItemsCount[i]++
		}
		if actions[i] == agent.ACTION_PICKUP && rewards[i] > 0 {
			sim.PickUpCount[i]++
		}
		if actions[i] == agent.ACTION_CLEAR && rewards[i] > 0 {
			sim.ClearCount[i]++
		}
	}
}

func (sim *Simulator) Dump() {
	fmt.Printf("TURN %d:\n", sim.Turn)
	mapData := [][]byte{}
	for _, row := range sim.MapData {
		mapData = append(mapData, []byte(row))
	}
	for i, agent := range sim.Agents {
		mapData[agent.Pos.R][agent.Pos.C] = byte('0' + i)
	}
	for _, row := range mapData {
		fmt.Println(string(row))
	}
	fmt.Println("[RESERVED]")
	fmt.Printf("%v\n", sim.Reserved)
	fmt.Println("[ITEMS]")
	fmt.Printf("%v\n", sim.Items)
	for i, agent := range sim.Agents {
		fmt.Printf("[AGENT %d]\n", agent.Id)
		if len(sim.LastActions) > 0 {
			fmt.Printf("last action: %s\n", sim.LastActions[i].ToStr())
		}
		fmt.Printf("pos: %v\n", agent.Pos)
		fmt.Printf("items count: %d ", sim.ItemsCount[i])
		fmt.Printf("pickup count: %d ", sim.PickUpCount[i])
		fmt.Printf("clear count: %d\n", sim.ClearCount[i])
		fmt.Printf("has item: %v\n", agent.HasItem)
	}
}
