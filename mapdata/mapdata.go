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
	NextPos      [][][]Pos
	ValidActions [][]agentaction.Actions
	MinDist      [][][][]int
}

func New(text []string) *MapData {
	h, w := len(text), len(text[0])
	var allPos []Pos
	var depotPos Pos
	nextPos := make([][][]Pos, h)
	validActions := make([][]agentaction.Actions, h)
	minDist := make([][][][]int, h)
	for r := 0; r < h; r++ {
		nextPos[r] = make([][]Pos, w)
		validActions[r] = make([]agentaction.Actions, w)
		minDist[r] = make([][][]int, w)
		for c := 0; c < w; c++ {
			if text[r][c] == '#' {
				continue
			}
			if text[r][c] == 'D' {
				depotPos = Pos{r, c}
			} else {
				allPos = append(allPos, Pos{r, c})
			}
			nextPos[r][c] = make([]Pos, agentaction.COUNT)
			nextPos[r][c][agentaction.STAY] = Pos{R: r, C: c}
			nextPos[r][c][agentaction.PICKUP] = Pos{R: r, C: c}
			nextPos[r][c][agentaction.CLEAR] = Pos{R: r, C: c}
			actions := agentaction.Actions{agentaction.STAY}
			if r > 0 && text[r-1][c] != '#' {
				nextPos[r][c][agentaction.UP] = Pos{R: r - 1, C: c}
				actions = append(actions, agentaction.UP)
			}
			if r+1 < h && text[r+1][c] != '#' {
				nextPos[r][c][agentaction.DOWN] = Pos{R: r + 1, C: c}
				actions = append(actions, agentaction.DOWN)
			}
			if c > 0 && text[r][c-1] != '#' {
				nextPos[r][c][agentaction.LEFT] = Pos{R: r, C: c - 1}
				actions = append(actions, agentaction.LEFT)
			}
			if c+1 < w && text[r][c+1] != '#' {
				nextPos[r][c][agentaction.RIGHT] = Pos{R: r, C: c + 1}
				actions = append(actions, agentaction.RIGHT)
			}
			validActions[r][c] = actions
			minDist[r][c] = bfs(text, h, w, Pos{R: r, C: c})
		}
	}

	return &MapData{
		Text:         text,
		H:            h,
		W:            w,
		AllPos:       allPos,
		DepotPos:     depotPos,
		NextPos:      nextPos,
		ValidActions: validActions,
		MinDist:      minDist,
	}
}

func bfs(text []string, h int, w int, startPos Pos) [][]int {
	dr := []int{-1, 0, 1, 0}
	dc := []int{0, 1, 0, -1}
	minDist := [][]int{}
	for i := 0; i < h; i++ {
		minDist = append(minDist, make([]int, w))
		for j := 0; j < w; j++ {
			minDist[i][j] = -1
		}
	}
	minDist[startPos.R][startPos.C] = 0
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
			if minDist[nr][nc] == -1 {
				minDist[nr][nc] = minDist[r][c] + 1
				que = append(que, nxt)
			}
		}
	}
	return minDist
}
