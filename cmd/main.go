package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"sync"

	"github.com/Div9851/new-warehouse-sim/config"
	"github.com/Div9851/new-warehouse-sim/mapdata"
	"github.com/Div9851/new-warehouse-sim/sim"
)

func calcAvgVar(s []float64) (float64, float64) {
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

func loadMapData(path string) (*mapdata.MapData, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("can't open `%s` (%s)", path, err)
	}
	defer f.Close()

	lines := []string{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if scanner.Err() != nil {
		return nil, fmt.Errorf("can't read `%s` (%s)", path, scanner.Err())
	}
	return mapdata.New(lines), nil
}

func loadConfig(path string) (*config.Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("can't open `%s` (%s)", path, err)
	}
	defer f.Close()

	b, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("can't read `%s` (%s)", path, err)
	}

	var config config.Config
	err = json.Unmarshal(b, &config)
	if err != nil {
		return nil, fmt.Errorf("can't decode `%s` (%s)", path, err)
	}
	return &config, nil
}

func main() {
	var (
		Run         = flag.Int("run", 1, "number of runs")
		mapDataFile = flag.String("mapdata-file", "", "path of mapdata file")
		configFile  = flag.String("config-file", "", "path of config file")
		verbose     = flag.Bool("verbose", false, "verbosity")
	)

	flag.Parse()

	mapData, err := loadMapData(*mapDataFile)
	if err != nil {
		panic(err)
	}
	config, err := loadConfig(*configFile)
	if err != nil {
		panic(err)
	}
	itemsCountHistory := make([][]float64, config.NumAgents)
	clearCountHistory := make([][]float64, config.NumAgents)
	clearRateHistory := make([][]float64, config.NumAgents)
	for i := 0; i < config.NumAgents; i++ {
		itemsCountHistory[i] = make([]float64, *Run)
		clearCountHistory[i] = make([]float64, *Run)
		clearRateHistory[i] = make([]float64, *Run)
	}
	var wg sync.WaitGroup
	for run := 0; run < *Run; run++ {
		wg.Add(1)
		go func(run int) {
			fmt.Printf("--- run %d start ---\n", run)
			sim := sim.New(mapData, config, *verbose, config.RandSeed+int64(run))
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
	totalItemsCountHistory := make([]float64, *Run)
	totalClearCountHistory := make([]float64, *Run)
	totalClearRateHistory := make([]float64, *Run)
	for i := 0; i < *Run; i++ {
		for j := 0; j < config.NumAgents; j++ {
			totalItemsCountHistory[i] += itemsCountHistory[j][i]
			totalClearCountHistory[i] += clearCountHistory[j][i]
		}
		totalClearRateHistory[i] = totalClearCountHistory[i] / totalItemsCountHistory[i]
	}
	fmt.Println("--items count--")
	for i := 0; i < config.NumAgents; i++ {
		average, variance := calcAvgVar(itemsCountHistory[i])
		fmt.Printf("AGENT %d: avg. %f var. %f\n", i, average, variance)
	}
	{
		average, variance := calcAvgVar(totalItemsCountHistory)
		fmt.Printf("TOTAL: avg. %f var. %f\n", average, variance)
	}
	fmt.Println("--clear count--")
	for i := 0; i < config.NumAgents; i++ {
		average, variance := calcAvgVar(clearCountHistory[i])
		fmt.Printf("AGENT %d: avg. %f var. %f\n", i, average, variance)
	}
	{
		average, variance := calcAvgVar(totalClearCountHistory)
		fmt.Printf("TOTAL: avg. %f var. %f\n", average, variance)
	}
	fmt.Println("--clear rate--")
	for i := 0; i < config.NumAgents; i++ {
		average, variance := calcAvgVar(clearRateHistory[i])
		fmt.Printf("AGENT %d: avg. %f var. %f\n", i, average, variance)
	}
	{
		average, variance := calcAvgVar(totalClearRateHistory)
		fmt.Printf("TOTAL: avg. %f var. %f\n", average, variance)
	}
}
