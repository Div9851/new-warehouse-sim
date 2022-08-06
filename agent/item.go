package agent

import "github.com/Div9851/new-warehouse-sim/mapdata"

type Item map[mapdata.Pos]int

type Items []Item

type ItemsDiff Items
