package sim

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"

	"github.com/Div9851/new-warehouse-sim/agentaction"
	"github.com/Div9851/new-warehouse-sim/agentstate"
	"github.com/Div9851/new-warehouse-sim/config"
	"github.com/Div9851/new-warehouse-sim/mapdata"
	"github.com/Div9851/new-warehouse-sim/mcts"
)

type Bid struct {
	Id       int
	SubGoals []mapdata.Pos
	Value    float64
}

func GetSubGoals(state agentstate.State, items map[mapdata.Pos]int, reserved map[mapdata.Pos]int, mapData *mapdata.MapData) ([]mapdata.Pos, int) {
	que := []mapdata.Pos{state.Pos}
	visited := make(map[mapdata.Pos]struct{})
	visited[state.Pos] = struct{}{}
	prevPos := make(map[mapdata.Pos]mapdata.Pos)
	last := mapdata.NonePos
	for len(que) > 0 {
		cur := que[0]
		que = que[1:]
		if (!state.HasItem && items[cur] > 0) || (state.HasItem && cur == mapData.DepotPos) {
			last = cur
			break
		}
		for _, action := range mapData.ValidActions[cur.R][cur.C] {
			nxt := mapData.NextPos[cur.R][cur.C][action]
			if _, exist := visited[nxt]; exist {
				continue
			}
			if _, exist := reserved[nxt]; exist {
				continue
			}
			que = append(que, nxt)
			visited[nxt] = struct{}{}
			prevPos[nxt] = cur
		}
	}
	if last == mapdata.NonePos {
		return nil, math.MaxInt
	}
	subGoals := []mapdata.Pos{}
	cur := last
	turns := 0
	for cur != state.Pos {
		if _, exist := mapData.SubGoals[cur]; exist || cur == last {
			subGoals = append(subGoals, cur)
		}
		cur = prevPos[cur]
		turns++
	}
	for i := 0; i*2 < len(subGoals); i++ {
		subGoals[i], subGoals[len(subGoals)-i-1] = subGoals[len(subGoals)-i-1], subGoals[i]
	}
	return subGoals, turns
}

type Simulator struct {
	Turn           int
	States         agentstate.States
	Items          []map[mapdata.Pos]int
	Budgets        []float64
	LastActions    agentaction.Actions
	ItemsCount     []int
	PickUpCount    []int
	ClearCount     []int
	SubGoals       [][]mapdata.Pos
	Reserved       map[mapdata.Pos]int
	MapData        *mapdata.MapData
	SimRandGen     *rand.Rand
	PlannerRandGen *rand.Rand
	Verbose        bool
}

func New(mapData *mapdata.MapData, seed int64, verbose bool) *Simulator {
	simRandGen := rand.New(rand.NewSource(seed))
	plannerRandGen := rand.New(rand.NewSource(simRandGen.Int63()))
	states := agentstate.States{}
	items := []map[mapdata.Pos]int{}
	budgets := []float64{}
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
		budgets = append(budgets, config.InitialBudget)
	}
	itemsCount := make([]int, config.NumAgents)
	pickUpCount := make([]int, config.NumAgents)
	clearCount := make([]int, config.NumAgents)
	return &Simulator{
		Turn:           0,
		States:         states,
		Items:          items,
		Budgets:        budgets,
		ItemsCount:     itemsCount,
		PickUpCount:    pickUpCount,
		ClearCount:     clearCount,
		SubGoals:       subGoals,
		Reserved:       reserved,
		MapData:        mapData,
		SimRandGen:     simRandGen,
		PlannerRandGen: plannerRandGen,
		Verbose:        verbose,
	}
}

func (sim *Simulator) Run() ([]int, []int, []int) {
	var nodePool = &sync.Pool{
		New: func() interface{} {
			return mcts.NewNode()
		},
	}
	for {
		if sim.Verbose {
			sim.Dump()
		}
		if sim.Turn == config.LastTurn {
			break
		}
		// プランニングフェーズ
		planner := mcts.New(sim.MapData, sim.PlannerRandGen, nodePool)
		for iter := 0; iter < config.NumIters; iter++ {
			planner.Update(sim.Turn, sim.States, sim.Items, sim.SubGoals)
		}
		actions := planner.BestActions(sim.States, sim.Items)
		// 予約フェーズ
		bids := []Bid{}
		for id := 0; id < config.NumAgents; id++ {
			// 既に予約している頂点があるなら新たな予約はしない
			if len(sim.SubGoals[id]) > 0 {
				continue
			}
			cur := sim.States[id]
			mctsTurns := math.MaxInt
			for depth := 0; depth < len(planner.Nodes[id]); depth++ {
				node := planner.Nodes[id][depth][cur]
				if node == nil {
					break
				}
				validActions := mcts.GetValidActions(cur, sim.Items[id], sim.MapData)
				action := node.BestAction(validActions)
				if action == agentaction.PICKUP || action == agentaction.CLEAR {
					mctsTurns = depth
					break
				}
				cur.Pos = sim.MapData.NextPos[cur.Pos.R][cur.Pos.C][action]
			}
			subGoals, greedyTurns := GetSubGoals(sim.States[id], sim.Items[id], sim.Reserved, sim.MapData)
			if len(subGoals) == 0 || greedyTurns >= mctsTurns {
				continue
			}
			bids = append(bids, Bid{
				Id:       id,
				SubGoals: subGoals,
				Value:    sim.Budgets[id] * (1 - float64(greedyTurns)/float64(mctsTurns)),
			})
		}
		sort.Slice(bids, func(i, j int) bool { return bids[i].Value > bids[j].Value })
	L:
		for _, bid := range bids {
			for _, subGoal := range bid.SubGoals {
				if _, exist := sim.Reserved[subGoal]; exist {
					continue L
				}
			}
			for _, subGoal := range bid.SubGoals {
				sim.Reserved[subGoal] = bid.Id
			}
			sim.SubGoals[bid.Id] = bid.SubGoals
			sim.Budgets[bid.Id] -= bid.Value
		}
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
			sim.Budgets[i] += config.IncreaseBudget
		}
		// PICKUP や CLEAR は可能なときにしか選ばないと仮定
		if actions[i] == agentaction.PICKUP {
			sim.PickUpCount[i]++
			for len(sim.SubGoals[i]) > 0 {
				delete(sim.Reserved, sim.SubGoals[i][0])
				sim.SubGoals[i] = sim.SubGoals[i][1:]
			}
		}
		if actions[i] == agentaction.CLEAR {
			sim.ClearCount[i]++
			for len(sim.SubGoals[i]) > 0 {
				delete(sim.Reserved, sim.SubGoals[i][0])
				sim.SubGoals[i] = sim.SubGoals[i][1:]
			}
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
		fmt.Printf("budget: %f\n", sim.Budgets[i])
		fmt.Printf("sub goals: %v\n", sim.SubGoals[i])
		fmt.Printf("items count: %d ", sim.ItemsCount[i])
		fmt.Printf("pickup count: %d ", sim.PickUpCount[i])
		fmt.Printf("clear count: %d\n", sim.ClearCount[i])
		fmt.Printf("has item: %v\n", state.HasItem)
	}
}
