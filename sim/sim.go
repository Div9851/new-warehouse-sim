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
		if config.EnableCooperation {
			depotPos := sim.MapData.DepotPos
			minDist := sim.MapData.MinDist
			load := make([]int, config.NumAgents)
			avgLoad := 0.0
			for id := 0; id < config.NumAgents; id++ {
				for pos, cnt := range sim.Items[id] {
					load[id] += minDist[depotPos.R][depotPos.C][pos.R][pos.C] * cnt
				}
				if sim.States[id].HasItem {
					pos := sim.States[id].Pos
					load[id] += minDist[depotPos.R][depotPos.C][pos.R][pos.C]
				}
				avgLoad += float64(load[id])
			}
			avgLoad /= float64(config.NumAgents)
			// 依頼フェーズ
			requests := []Request{}
			for id := 0; id < config.NumAgents; id++ {
				// 負荷が平均以下なら依頼しない
				if float64(load[id]) <= avgLoad {
					continue
				}
				diff := float64(load[id]) - avgLoad
				reqPos := mapdata.NonePos
				mi := math.MaxInt
				for pos := range sim.Items[id] {
					// 人から引き受けた依頼を再依頼はしない
					if pos == sim.AcceptedRequest[id].Pos {
						continue
					}
					d := minDist[pos.R][pos.C][depotPos.R][depotPos.C]
					if float64(d) > diff {
						continue
					}
					if mi > d {
						mi = d
						reqPos = pos
					}
				}
				if reqPos == mapdata.NonePos {
					continue
				}
				requests = append(requests, Request{
					From: id,
					Pos:  reqPos,
				})
			}
			// 引き受けフェーズ
			acceptedId := make([][]int, len(requests))
			for id := 0; id < config.NumAgents; id++ {
				// 既に別の依頼を引き受けているなら引き受けない
				if sim.AcceptedRequest[id] != dummyRequest {
					continue
				}
				// 負荷が平均以上なら引き受けない
				if float64(load[id]) >= avgLoad {
					continue
				}
				diff := avgLoad - float64(load[id])
				pos := sim.States[id].Pos
				chosenId := -1
				mi := math.MaxInt
				for reqId, req := range requests {
					d := minDist[depotPos.R][depotPos.C][req.Pos.R][req.Pos.C]
					if float64(d) > diff {
						continue
					}
					d = minDist[pos.R][pos.C][req.Pos.R][req.Pos.C]
					if mi > d {
						mi = d
						chosenId = reqId
					}
				}
				if chosenId == -1 {
					continue
				}
				acceptedId[chosenId] = append(acceptedId[chosenId], id)
			}
			// 依頼先決定フェーズ
			for reqId, req := range requests {
				chosenId := -1
				mi := math.MaxInt
				for _, id := range acceptedId[reqId] {
					pos := sim.States[id].Pos
					d := minDist[pos.R][pos.C][req.Pos.R][req.Pos.C]
					if mi > d {
						mi = d
						chosenId = id
					}
				}
				if chosenId == -1 {
					continue
				}
				sim.Items[req.From][req.Pos]--
				if sim.Items[req.From][req.Pos] == 0 {
					delete(sim.Items[req.From], req.Pos)
				}
				sim.Items[chosenId][req.Pos]++
				sim.AcceptedRequest[chosenId] = req
			}
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
