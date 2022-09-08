package main

import (
	"flag"
	"fmt"
	"sync"

	"github.com/Div9851/new-warehouse-sim/config"
	"github.com/Div9851/new-warehouse-sim/mapdata"
	"github.com/Div9851/new-warehouse-sim/sim"
)

func calc(s []float64) (float64, float64) {
	n := len(s)
	var (
		average  float64
		variance float64
	)
	for _, x := range s {
		average += x
		variance += x * x
	}
	average /= float64(n)
	variance /= float64(n)
	variance -= average * average
	return average, variance
}

func main() {
	var (
		numRun  = flag.Int("numrun", 1, "number of runs")
		verbose = flag.Bool("verbose", false, "show all logs")
	)
	flag.Parse()
	text := []string{
		"...#...",
		".#.#.#.",
		".#.#.#.",
		"D......",
		".##.##.",
		".##.##.",
		".##.##.",
	}
	mapData := mapdata.New(text)
	itemsCountHistory := make([][]float64, config.NumAgents)
	clearCountHistory := make([][]float64, config.NumAgents)
	clearRateHistory := make([][]float64, config.NumAgents)
	for i := 0; i < config.NumAgents; i++ {
		itemsCountHistory[i] = make([]float64, *numRun)
		clearCountHistory[i] = make([]float64, *numRun)
		clearRateHistory[i] = make([]float64, *numRun)
	}
	var wg sync.WaitGroup
	for run := 0; run < *numRun; run++ {
		wg.Add(1)
		go func(run int) {
			fmt.Printf("--- run %d start ---\n", run)
			sim := sim.New(mapData, config.RandSeed+int64(run), *verbose)
			itemsCount, _, clearCount := sim.Run()
			for i := 0; i < config.NumAgents; i++ {
				r := float64(clearCount[i]) / float64(itemsCount[i])
				itemsCountHistory[i][run] = float64(itemsCount[i])
				clearCountHistory[i][run] = float64(clearCount[i])
				clearRateHistory[i][run] = r
			}
			fmt.Printf("--- run %d end ---\n", run)
			wg.Done()
		}(run)
	}
	wg.Wait()
	fmt.Println("--items count--")
	for i := 0; i < config.NumAgents; i++ {
		average, variance := calc(itemsCountHistory[i])
		fmt.Printf("AGENT %d: avg. %f var. %f\n", i, average, variance)
	}
	fmt.Println("--clear count--")
	for i := 0; i < config.NumAgents; i++ {
		average, variance := calc(clearCountHistory[i])
		fmt.Printf("AGENT %d: avg. %f var. %f\n", i, average, variance)
	}
	fmt.Println("--clear rate--")
	for i := 0; i < config.NumAgents; i++ {
		average, variance := calc(clearRateHistory[i])
		fmt.Printf("AGENT %d: avg. %f var. %f\n", i, average, variance)
	}
}
