package sim

import (
	"fmt"
	"math/rand"
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
	PickedAt        []mapdata.Pos
	AcceptedRequest []Request
	Balance         [][]int
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
	pickedAt := []mapdata.Pos{}
	acceptedRequest := []Request{}
	balance := make([][]int, config.NumAgents)
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
		pickedAt = append(pickedAt, mapdata.NonePos)
		acceptedRequest = append(acceptedRequest, dummyRequest)
		balance[i] = make([]int, config.NumAgents)
	}
	itemsCount := make([]int, config.NumAgents)
	pickUpCount := make([]int, config.NumAgents)
	clearCount := make([]int, config.NumAgents)
	return &Simulator{
		Turn:            0,
		States:          states,
		Items:           items,
		PickedAt:        pickedAt,
		AcceptedRequest: acceptedRequest,
		Balance:         balance,
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
		// 依頼フェーズ
		requests := []Request{}
		for id := 0; id < config.NumAgents; id++ {
			agentPos := sim.States[id].Pos
			farthestPos := mapdata.NonePos
			maxDist := -1
			for pos := range sim.Items[id] {
				// 人から引き受けた依頼を再依頼はしない
				if pos == sim.AcceptedRequest[id].Pos {
					continue
				}
				dist := sim.MapData.MinDist[agentPos.R][agentPos.C][pos.R][pos.C]
				if maxDist < dist {
					maxDist = dist
					farthestPos = pos
				}
			}
			if farthestPos == mapdata.NonePos {
				continue
			}
			requests = append(requests, Request{
				From: id,
				Pos:  farthestPos,
			})
		}
		// 引き受けフェーズ
		acceptedId := make([][]int, len(requests))
		for id := 0; id < config.NumAgents; id++ {
			if sim.States[id].HasItem || len(sim.Items[id]) > 0 {
				continue
			}
			agentPos := sim.States[id].Pos
			bestReqId := -1
			for reqId, req := range requests {
				if req.From == id {
					continue
				}
				if bestReqId == -1 {
					bestReqId = reqId
					continue
				}
				bestReq := requests[bestReqId]
				if sim.Balance[id][bestReq.From] != sim.Balance[id][req.From] {
					if sim.Balance[id][bestReq.From] < sim.Balance[id][req.From] {
						bestReqId = reqId
					}
					continue
				}
				if sim.MapData.MinDist[agentPos.R][agentPos.C][bestReq.Pos.R][bestReq.Pos.C] > sim.MapData.MinDist[agentPos.R][agentPos.C][req.Pos.R][req.Pos.C] {
					bestReqId = reqId
				}
			}
			if bestReqId == -1 {
				continue
			}
			acceptedId[bestReqId] = append(acceptedId[bestReqId], id)
		}
		// 依頼先決定フェーズ
		for reqId, req := range requests {
			bestId := -1
			for _, id := range acceptedId[reqId] {
				if bestId == -1 {
					bestId = id
					continue
				}
				bestPos := sim.States[bestId].Pos
				agentPos := sim.States[id].Pos
				if sim.MapData.MinDist[req.Pos.R][req.Pos.C][bestPos.R][bestPos.C] > sim.MapData.MinDist[req.Pos.R][req.Pos.C][agentPos.R][agentPos.C] {
					bestId = id
				}
			}
			if bestId == -1 {
				continue
			}
			sim.Items[req.From][req.Pos]--
			if sim.Items[req.From][req.Pos] == 0 {
				delete(sim.Items[req.From], req.Pos)
			}
			sim.Items[bestId][req.Pos]++
			sim.Balance[req.From][bestId]++
			sim.Balance[bestId][req.From]--
			sim.AcceptedRequest[bestId] = req
		}
		// プランニングフェーズ
		planners := make([]*fduct.Planner, config.NumAgents)
		actions := make(agentaction.Actions, config.NumAgents)
		var wg sync.WaitGroup
		for id := 0; id < config.NumAgents; id++ {
			wg.Add(1)
			planners[id] = fduct.New(sim.MapData, sim.PlannerRandGens[id], nodePool, 0)
			go func(id int) {
				for iter := 0; iter < config.NumIters; iter++ {
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
	free := make([]bool, config.NumAgents)
	nxtStates, _, newItem := agentstate.Next(sim.States, actions, free, sim.Items, sim.MapData, sim.SimRandGen, config.NewItemProb)
	sim.States = nxtStates
	for i := 0; i < config.NumAgents; i++ {
		if newItem[i] {
			sim.ItemsCount[i]++
		}
		// PICKUP や CLEAR は可能なときにしか選ばないと仮定
		if actions[i] == agentaction.PICKUP {
			if sim.States[i].Pos == sim.AcceptedRequest[i].Pos {
				sim.PickUpCount[sim.AcceptedRequest[i].From]++
			} else {
				sim.PickUpCount[i]++
			}
			sim.PickedAt[i] = sim.States[i].Pos
		}
		if actions[i] == agentaction.CLEAR {
			if sim.PickedAt[i] == sim.AcceptedRequest[i].Pos {
				sim.ClearCount[sim.AcceptedRequest[i].From]++
				sim.AcceptedRequest[i] = dummyRequest
			} else {
				sim.ClearCount[i]++
			}
			sim.PickedAt[i] = mapdata.NonePos
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
		fmt.Printf("picked at: %v\n", sim.PickedAt[i])
		fmt.Printf("accepted request: %v\n", sim.AcceptedRequest[i])
	}
}
