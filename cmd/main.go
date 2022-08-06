package main

import (
	"github.com/Div9851/new-warehouse-sim/agent"
	"github.com/Div9851/new-warehouse-sim/config"
	"github.com/Div9851/new-warehouse-sim/mcts"
	"github.com/Div9851/new-warehouse-sim/sim"
)

func main() {
	mapData := []string{"...#...", ".#.#.#.", ".#.#.#.", "D......", ".##.##.", ".##.##.", ".##.##."}
	sim := sim.New(config.NumAgents, mapData, 123)
	for {
		sim.Dump()
		if sim.Turn == config.LastTurn {
			break
		}
		var actions agent.Actions
		for i := 0; i < config.NumAgents; i++ {
			node := mcts.New(sim.Turn, 0, sim.Agents, i, sim.MapData, sim.AllPos, sim.Reserved, sim.RandGens[i])
			actions = append(actions, mcts.MCTS(node, sim.Items))
		}
		sim.Next(actions)
	}
}
