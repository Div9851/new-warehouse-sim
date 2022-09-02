package mapdata

import (
	"github.com/Div9851/new-warehouse-sim/agentaction"
)

type Pos struct {
	R, C int
}

var NonePos Pos = Pos{R: -1, C: -1}

type MapData struct {
	Text         []string
	H, W         int
	AllPos       []Pos
	DepotPos     Pos
	ValidActions map[Pos]agentaction.Actions
	MinDist      map[Pos]map[Pos]int
}

func New(text []string) *MapData {
	h, w := len(text), len(text[0])
	var allPos []Pos
	var depotPos Pos
	validActions := make(map[Pos]agentaction.Actions)
	minDist := make(map[Pos]map[Pos]int)
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			if text[r][c] == '#' {
				continue
			}
			if text[r][c] == 'D' {
				depotPos = Pos{r, c}

			} else {
				allPos = append(allPos, Pos{r, c})
			}
			actions := agentaction.Actions{}
			if r > 0 && text[r-1][c] != '#' {
				actions = append(actions, agentaction.UP)
			}
			if r+1 < h && text[r+1][c] != '#' {
				actions = append(actions, agentaction.DOWN)
			}
			if c > 0 && text[r][c-1] != '#' {
				actions = append(actions, agentaction.LEFT)
			}
			if c+1 < w && text[r][c+1] != '#' {
				actions = append(actions, agentaction.RIGHT)
			}
			validActions[Pos{R: r, C: c}] = actions
			minDist[Pos{R: r, C: c}] = bfs(text, h, w, Pos{R: r, C: c})
		}
	}

	return &MapData{
		Text:         text,
		H:            h,
		W:            w,
		AllPos:       allPos,
		DepotPos:     depotPos,
		ValidActions: validActions,
		MinDist:      minDist,
	}
}

func bfs(text []string, h int, w int, startPos Pos) map[Pos]int {
	dr := []int{-1, 0, 1, 0}
	dc := []int{0, 1, 0, -1}
	minDist := make(map[Pos]int)
	minDist[startPos] = 0
	que := []Pos{startPos}
	for len(que) > 0 {
		cur := que[0]
		r, c := cur.R, cur.C
		que = que[1:]
		for i := 0; i < 4; i++ {
			nr, nc := r+dr[i], c+dc[i]
			nxt := Pos{R: nr, C: nc}
			if 0 > nr || nr >= h || 0 > nc || nc >= w || text[nr][nc] == '#' {
				continue
			}
			if _, ok := minDist[nxt]; !ok {
				minDist[nxt] = minDist[cur] + 1
				que = append(que, nxt)
			}
		}
	}
	return minDist
}
