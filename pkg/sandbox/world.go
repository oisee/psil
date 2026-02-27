package sandbox

import "math/rand"

// Tile types (4-bit, stored in low nibble)
const (
	TileEmpty = iota
	TileWall
	TileFood
	TileWater
)

// Tile packs type (low 4 bits) + occupant ID (high 4 bits)
type Tile byte

func MakeTile(typ byte, occupant byte) Tile {
	return Tile((occupant&0x0F)<<4 | (typ & 0x0F))
}

func (t Tile) Type() byte     { return byte(t) & 0x0F }
func (t Tile) Occupant() byte { return byte(t) >> 4 }

// World is a 2D tile grid with NPCs.
type World struct {
	Size int // width and height (square)
	Grid []Tile
	NPCs []*NPC
	Tick int

	// Config
	FoodRate    float64 // probability of food spawn per tick
	MaxFood     int     // max food tiles on map
	Rng         *rand.Rand
	NextID      byte
	FoodSpawned int
}

// NewWorld creates a Size×Size world.
func NewWorld(size int, rng *rand.Rand) *World {
	w := &World{
		Size:     size,
		Grid:     make([]Tile, size*size),
		NPCs:     make([]*NPC, 0, 32),
		FoodRate: 0.3,
		MaxFood:  size, // roughly 1 food per row
		Rng:      rng,
		NextID:   1,
	}
	return w
}

func (w *World) idx(x, y int) int {
	return y*w.Size + x
}

func (w *World) InBounds(x, y int) bool {
	return x >= 0 && x < w.Size && y >= 0 && y < w.Size
}

func (w *World) TileAt(x, y int) Tile {
	if !w.InBounds(x, y) {
		return Tile(TileWall)
	}
	return w.Grid[w.idx(x, y)]
}

func (w *World) SetTile(x, y int, t Tile) {
	if w.InBounds(x, y) {
		w.Grid[w.idx(x, y)] = t
	}
}

func (w *World) Spawn(npc *NPC) bool {
	if npc.ID == 0 {
		npc.ID = w.NextID
		w.NextID++
		if w.NextID > 15 { // 4-bit occupant ID
			w.NextID = 1
		}
	}
	// Find empty tile if position occupied
	if w.TileAt(npc.X, npc.Y).Type() != TileEmpty || w.TileAt(npc.X, npc.Y).Occupant() != 0 {
		// Try random placement
		for tries := 0; tries < 100; tries++ {
			x := w.Rng.Intn(w.Size)
			y := w.Rng.Intn(w.Size)
			t := w.TileAt(x, y)
			if t.Type() == TileEmpty && t.Occupant() == 0 {
				npc.X = x
				npc.Y = y
				break
			}
		}
	}
	w.SetTile(npc.X, npc.Y, MakeTile(TileEmpty, npc.ID))
	w.NPCs = append(w.NPCs, npc)
	return true
}

func (w *World) Remove(id byte) {
	for i, npc := range w.NPCs {
		if npc.ID == id {
			w.SetTile(npc.X, npc.Y, MakeTile(TileEmpty, 0))
			w.NPCs = append(w.NPCs[:i], w.NPCs[i+1:]...)
			return
		}
	}
}

func (w *World) FoodCount() int {
	count := 0
	for _, t := range w.Grid {
		if t.Type() == TileFood {
			count++
		}
	}
	return count
}

func (w *World) RespawnFood() {
	if w.FoodCount() >= w.MaxFood {
		return
	}
	if w.Rng.Float64() > w.FoodRate {
		return
	}
	// Place 1-3 food items
	n := 1 + w.Rng.Intn(3)
	for i := 0; i < n && w.FoodCount() < w.MaxFood; i++ {
		for tries := 0; tries < 50; tries++ {
			x := w.Rng.Intn(w.Size)
			y := w.Rng.Intn(w.Size)
			t := w.TileAt(x, y)
			if t.Type() == TileEmpty && t.Occupant() == 0 {
				w.SetTile(x, y, MakeTile(TileFood, 0))
				w.FoodSpawned++
				break
			}
		}
	}
}

// NearestFood returns Manhattan distance to nearest food tile, or 31 if none.
func (w *World) NearestFood(x, y int) int {
	best := 31
	for fy := 0; fy < w.Size; fy++ {
		for fx := 0; fx < w.Size; fx++ {
			if w.TileAt(fx, fy).Type() == TileFood {
				d := abs(fx-x) + abs(fy-y)
				if d < best {
					best = d
				}
			}
		}
	}
	return best
}

// NearestNPC returns Manhattan distance to nearest other NPC, or 31 if none.
func (w *World) NearestNPC(x, y int, excludeID byte) int {
	best := 31
	for _, npc := range w.NPCs {
		if npc.ID == excludeID || !npc.Alive() {
			continue
		}
		d := abs(npc.X-x) + abs(npc.Y-y)
		if d > 0 && d < best {
			best = d
		}
	}
	return best
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
