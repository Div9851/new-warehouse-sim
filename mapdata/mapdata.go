package mapdata

type MapData []string

type Pos struct {
	R, C int
}

type PosTurn struct {
	R, C, Turn int
}

type RandGen func() Pos

func (mapdata MapData) Size() (h int, w int) {
	return len(mapdata), len(mapdata[0])
}

func (mapdata MapData) GetAllPos() []Pos {
	h, w := mapdata.Size()
	var allPos []Pos
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			if mapdata[r][c] == '.' {
				allPos = append(allPos, Pos{r, c})
			}
		}
	}
	return allPos
}
