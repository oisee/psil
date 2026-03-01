package sandbox

import "math/rand"

// Tile types (4-bit, stored in low nibble)
const (
	TileEmpty = iota
	TileWall
	TileFood
	TileWater
	TileTool
	TileWeapon
	TileTreasure
	TileCrystal // 7
	TileForge   // 8
	TilePoison  // 9 — deals damage when walked on
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
	ItemRate    float64 // probability of item spawn per tick
	MaxItems    int     // cap for item tiles on map
	Rng         *rand.Rand
	NextID      byte
	FoodSpawned int

	// Poison tile lifetimes: grid index → tick when placed
	PoisonTTL map[int]int
}

// NewWorld creates a Size×Size world.
func NewWorld(size int, rng *rand.Rand) *World {
	w := &World{
		Size:      size,
		Grid:      make([]Tile, size*size),
		NPCs:      make([]*NPC, 0, 32),
		FoodRate:  0.25,
		MaxFood:   size * 3 / 4,
		ItemRate:  0.05,
		MaxItems:  size / 4,
		Rng:       rng,
		NextID:    1,
		PoisonTTL: make(map[int]int),
	}

	// Place forges: max(3, size/8)
	numForges := size / 8
	if numForges < 3 {
		numForges = 3
	}
	for i := 0; i < numForges; i++ {
		for tries := 0; tries < 50; tries++ {
			x := rng.Intn(size)
			y := rng.Intn(size)
			if w.TileAt(x, y).Type() == TileEmpty {
				w.SetTile(x, y, MakeTile(TileForge, 0))
				break
			}
		}
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
	// Find valid tile if position occupied or blocked
	tileOk := func(t Tile) bool {
		typ := t.Type()
		return (typ == TileEmpty || typ == TileForge) && t.Occupant() == 0
	}
	if !tileOk(w.TileAt(npc.X, npc.Y)) {
		// Try random placement
		for tries := 0; tries < 100; tries++ {
			x := w.Rng.Intn(w.Size)
			y := w.Rng.Intn(w.Size)
			if tileOk(w.TileAt(x, y)) {
				npc.X = x
				npc.Y = y
				break
			}
		}
	}
	// Seed per-NPC RNG from ID and world tick
	npc.RngState = [3]byte{npc.ID, byte(w.Tick), byte(w.Tick >> 8)}

	// Preserve underlying tile type (e.g. forge)
	baseType := w.TileAt(npc.X, npc.Y).Type()
	w.SetTile(npc.X, npc.Y, MakeTile(baseType, npc.ID))
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
	// Winter: last quarter of day cycle (ticks 192-255), no food spawns
	if w.Tick%DayCycle >= DayCycle*3/4 {
		return
	}
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

// NearestNPCID returns the ID of the nearest other NPC, or 0 if none.
func (w *World) NearestNPCID(x, y int, excludeID byte) byte {
	best := 31
	bestID := byte(0)
	for _, npc := range w.NPCs {
		if npc.ID == excludeID || !npc.Alive() {
			continue
		}
		d := abs(npc.X-x) + abs(npc.Y-y)
		if d > 0 && d < best {
			best = d
			bestID = npc.ID
		}
	}
	return bestID
}

// directionToward returns the move direction (1=N,2=E,3=S,4=W) toward (tx,ty) from (fx,fy).
// Picks the axis with the larger delta. Returns 0 if same position.
func directionToward(fx, fy, tx, ty int) int {
	dx := tx - fx
	dy := ty - fy
	if dx == 0 && dy == 0 {
		return DirNone
	}
	// Pick the axis with the larger magnitude
	if abs(dy) >= abs(dx) {
		if dy < 0 {
			return DirNorth
		}
		return DirSouth
	}
	if dx > 0 {
		return DirEast
	}
	return DirWest
}

// NearestFoodDir returns the direction (1=N,2=E,3=S,4=W) toward nearest food, or 0.
func (w *World) NearestFoodDir(x, y int) int {
	best := 31
	bx, by := x, y
	for fy := 0; fy < w.Size; fy++ {
		for fx := 0; fx < w.Size; fx++ {
			if w.TileAt(fx, fy).Type() == TileFood {
				d := abs(fx-x) + abs(fy-y)
				if d < best {
					best = d
					bx, by = fx, fy
				}
			}
		}
	}
	if best == 31 {
		return DirNone
	}
	return directionToward(x, y, bx, by)
}

// NearestNPCDir returns the direction toward the nearest other NPC, or 0.
func (w *World) NearestNPCDir(x, y int, excludeID byte) int {
	best := 31
	bx, by := x, y
	for _, npc := range w.NPCs {
		if npc.ID == excludeID || !npc.Alive() {
			continue
		}
		d := abs(npc.X-x) + abs(npc.Y-y)
		if d > 0 && d < best {
			best = d
			bx, by = npc.X, npc.Y
		}
	}
	if best == 31 {
		return DirNone
	}
	return directionToward(x, y, bx, by)
}

// NearestItemDir returns the direction toward the nearest item tile, or 0.
func (w *World) NearestItemDir(x, y int) int {
	best := 31
	bx, by := x, y
	for fy := 0; fy < w.Size; fy++ {
		for fx := 0; fx < w.Size; fx++ {
			typ := w.TileAt(fx, fy).Type()
			if (typ >= TileTool && typ <= TileTreasure) || typ == TileCrystal {
				d := abs(fx-x) + abs(fy-y)
				if d < best {
					best = d
					bx, by = fx, fy
				}
			}
		}
	}
	if best == 31 {
		return DirNone
	}
	return directionToward(x, y, bx, by)
}

// ItemCount returns the number of item tiles (tool, weapon, treasure, crystal) on the map.
func (w *World) ItemCount() int {
	count := 0
	for _, t := range w.Grid {
		typ := t.Type()
		if (typ >= TileTool && typ <= TileTreasure) || typ == TileCrystal {
			count++
		}
	}
	return count
}

// RespawnItems spawns item tiles (tool, weapon, treasure) similar to RespawnFood.
func (w *World) RespawnItems() {
	if w.ItemCount() >= w.MaxItems {
		return
	}
	if w.Rng.Float64() > w.ItemRate {
		return
	}
	// Place 1 item (1-in-10 chance it's poison instead)
	for tries := 0; tries < 50; tries++ {
		x := w.Rng.Intn(w.Size)
		y := w.Rng.Intn(w.Size)
		t := w.TileAt(x, y)
		if t.Type() == TileEmpty && t.Occupant() == 0 {
			if w.Rng.Intn(10) == 0 {
				w.SetTile(x, y, MakeTile(TilePoison, 0))
				w.PoisonTTL[w.idx(x, y)] = w.Tick
			} else {
				var itemType byte
				if w.Rng.Intn(20) == 0 {
					itemType = TileCrystal // 1-in-20 chance
				} else {
					itemType = byte(TileTool + w.Rng.Intn(3))
				}
				w.SetTile(x, y, MakeTile(itemType, 0))
			}
			break
		}
	}
}

// NearestItem returns (Manhattan distance, tile type) of nearest item tile, or (31, 0) if none.
func (w *World) NearestItem(x, y int) (int, byte) {
	best := 31
	bestType := byte(0)
	for fy := 0; fy < w.Size; fy++ {
		for fx := 0; fx < w.Size; fx++ {
			typ := w.TileAt(fx, fy).Type()
			if (typ >= TileTool && typ <= TileTreasure) || typ == TileCrystal {
				d := abs(fx-x) + abs(fy-y)
				if d < best {
					best = d
					bestType = typ
				}
			}
		}
	}
	return best, bestType
}

// ItemCountByType returns the count of items of a given type, including held by NPCs and on tiles.
func (w *World) ItemCountByType(item byte) int {
	count := 0
	// Count held by NPCs
	for _, npc := range w.NPCs {
		if npc.Alive() && npc.Item == item {
			count++
		}
	}
	// Count on tiles (map item type to tile type)
	var tileType byte
	switch item {
	case ItemTool:
		tileType = TileTool
	case ItemWeapon:
		tileType = TileWeapon
	case ItemTreasure:
		tileType = TileTreasure
	case ItemCrystal:
		tileType = TileCrystal
	default:
		return count // crafted items only exist as held
	}
	for _, t := range w.Grid {
		if t.Type() == tileType {
			count++
		}
	}
	return count
}

// MarketValue returns the scarcity-based value of an item type.
// Formula: 10 * totalItems / thisTypeCount (higher when rare).
func (w *World) MarketValue(item byte) int {
	total := 0
	for _, npc := range w.NPCs {
		if npc.Alive() && npc.Item != ItemNone {
			total++
		}
	}
	total += w.ItemCount()
	if total == 0 {
		return 10
	}
	thisCount := w.ItemCountByType(item)
	if thisCount == 0 {
		return total * 10 // extremely rare
	}
	return 10 * total / thisCount
}

// NearestPoison returns Manhattan distance to nearest poison tile, or 31 if none.
func (w *World) NearestPoison(x, y int) int {
	best := 31
	for py := 0; py < w.Size; py++ {
		for px := 0; px < w.Size; px++ {
			if w.TileAt(px, py).Type() == TilePoison {
				d := abs(px-x) + abs(py-y)
				if d < best {
					best = d
				}
			}
		}
	}
	return best
}

// DecayPoison removes poison tiles that have existed for >= 200 ticks.
func (w *World) DecayPoison() {
	for idx, placedTick := range w.PoisonTTL {
		if w.Tick-placedTick >= 200 {
			y := idx / w.Size
			x := idx % w.Size
			if w.TileAt(x, y).Type() == TilePoison {
				w.SetTile(x, y, MakeTile(TileEmpty, 0))
			}
			delete(w.PoisonTTL, idx)
		}
	}
}

// Blight destroys ~50% of food tiles on the map.
func (w *World) Blight() {
	for y := 0; y < w.Size; y++ {
		for x := 0; x < w.Size; x++ {
			if w.TileAt(x, y).Type() == TileFood {
				if w.Rng.Intn(2) == 0 {
					w.SetTile(x, y, MakeTile(TileEmpty, 0))
				}
			}
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
