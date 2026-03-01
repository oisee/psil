package sandbox

import (
	"math"
	"math/rand"
)

// Tile types (full byte, 256 possible types)
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

// Tile is pure terrain — occupancy is tracked separately in OccGrid.
type Tile byte

func MakeTile(typ byte) Tile {
	return Tile(typ)
}

func (t Tile) Type() byte { return byte(t) }

// isFood returns true if the tile type is food.
func isFood(typ byte) bool { return typ == TileFood }

// isItem returns true if the tile type is an item (tool/weapon/treasure/crystal).
func isItem(typ byte) bool {
	return (typ >= TileTool && typ <= TileTreasure) || typ == TileCrystal
}

// World is a 2D tile grid with NPCs.
type World struct {
	Size int // width and height (square)
	Grid []Tile
	NPCs []*NPC
	Tick int

	// Occupancy grid: parallel to Grid, stores NPC ID (0 = empty)
	OccGrid []uint16
	// NPC lookup by ID
	npcByID map[uint16]*NPC

	// Cached tile counts (maintained by SetTile)
	foodCount int
	itemCount int

	// Config
	FoodRate    float64 // probability of food spawn per tick
	MaxFood     int     // max food tiles on map
	ItemRate    float64 // probability of item spawn per tick
	MaxItems    int     // cap for item tiles on map
	Rng         *rand.Rand
	NextID      uint16
	FoodSpawned int

	// Poison tile lifetimes: grid index → tick when placed
	PoisonTTL map[int]int
}

// NewWorld creates a Size×Size world.
func NewWorld(size int, rng *rand.Rand) *World {
	w := &World{
		Size:      size,
		Grid:      make([]Tile, size*size),
		OccGrid:   make([]uint16, size*size),
		NPCs:      make([]*NPC, 0, 32),
		npcByID:   make(map[uint16]*NPC),
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
				w.SetTile(x, y, MakeTile(TileForge))
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
	if !w.InBounds(x, y) {
		return
	}
	i := w.idx(x, y)
	old := w.Grid[i].Type()
	newTyp := t.Type()

	// Maintain cached counts
	if isFood(old) {
		w.foodCount--
	}
	if isItem(old) {
		w.itemCount--
	}
	if isFood(newTyp) {
		w.foodCount++
	}
	if isItem(newTyp) {
		w.itemCount++
	}

	w.Grid[i] = t
}

// OccAt returns the NPC ID occupying (x,y), or 0 if empty.
func (w *World) OccAt(x, y int) uint16 {
	if !w.InBounds(x, y) {
		return 0
	}
	return w.OccGrid[w.idx(x, y)]
}

// SetOcc sets the occupant ID at (x,y).
func (w *World) SetOcc(x, y int, id uint16) {
	if w.InBounds(x, y) {
		w.OccGrid[w.idx(x, y)] = id
	}
}

// ClearOcc clears the occupant at (x,y).
func (w *World) ClearOcc(x, y int) {
	if w.InBounds(x, y) {
		w.OccGrid[w.idx(x, y)] = 0
	}
}

// NPCByID returns the NPC with the given ID, or nil.
func (w *World) NPCByID(id uint16) *NPC {
	return w.npcByID[id]
}

func (w *World) Spawn(npc *NPC) bool {
	if npc.ID == 0 {
		npc.ID = w.NextID
		w.NextID++
	}
	// Find valid tile if position occupied or blocked
	tileOk := func(x, y int) bool {
		t := w.TileAt(x, y)
		typ := t.Type()
		return (typ == TileEmpty || typ == TileForge) && w.OccAt(x, y) == 0
	}
	if !tileOk(npc.X, npc.Y) {
		// Try random placement
		for tries := 0; tries < 100; tries++ {
			x := w.Rng.Intn(w.Size)
			y := w.Rng.Intn(w.Size)
			if tileOk(x, y) {
				npc.X = x
				npc.Y = y
				break
			}
		}
	}
	// Seed per-NPC RNG from ID and world tick
	npc.RngState = [3]byte{byte(npc.ID), byte(npc.ID>>8) ^ byte(w.Tick), byte(w.Tick >> 8)}

	w.SetOcc(npc.X, npc.Y, npc.ID)
	w.NPCs = append(w.NPCs, npc)
	w.npcByID[npc.ID] = npc
	return true
}

func (w *World) Remove(id uint16) {
	npc := w.npcByID[id]
	if npc == nil {
		return
	}
	w.ClearOcc(npc.X, npc.Y)
	delete(w.npcByID, id)
	for i, n := range w.NPCs {
		if n.ID == id {
			w.NPCs = append(w.NPCs[:i], w.NPCs[i+1:]...)
			return
		}
	}
}

func (w *World) FoodCount() int {
	return w.foodCount
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
			if w.TileAt(x, y).Type() == TileEmpty && w.OccAt(x, y) == 0 {
				w.SetTile(x, y, MakeTile(TileFood))
				w.FoodSpawned++
				break
			}
		}
	}
}

// scanManhattanRing calls fn for each cell at exactly Manhattan distance d from (cx,cy).
// fn returns true to stop scanning (found). Returns true if fn stopped early.
func (w *World) scanManhattanRing(cx, cy, d int, fn func(x, y int) bool) bool {
	if d == 0 {
		if w.InBounds(cx, cy) {
			return fn(cx, cy)
		}
		return false
	}
	// Walk the diamond perimeter: 4 edges, d cells each
	for i := 0; i < d; i++ {
		// Top-right edge: (cx+i, cy-d+i)
		if x, y := cx+i, cy-d+i; w.InBounds(x, y) {
			if fn(x, y) {
				return true
			}
		}
		// Right-bottom edge: (cx+d-i, cy+i)
		if x, y := cx+d-i, cy+i; w.InBounds(x, y) {
			if fn(x, y) {
				return true
			}
		}
		// Bottom-left edge: (cx-i, cy+d-i)
		if x, y := cx-i, cy+d-i; w.InBounds(x, y) {
			if fn(x, y) {
				return true
			}
		}
		// Left-top edge: (cx-d+i, cy-i)
		if x, y := cx-d+i, cy-i; w.InBounds(x, y) {
			if fn(x, y) {
				return true
			}
		}
	}
	return false
}

const maxSearchRadius = 31

// NearestFood returns Manhattan distance to nearest food tile, or 31 if none.
func (w *World) NearestFood(x, y int) int {
	for d := 0; d <= maxSearchRadius; d++ {
		found := false
		w.scanManhattanRing(x, y, d, func(fx, fy int) bool {
			if w.TileAt(fx, fy).Type() == TileFood {
				found = true
				return true
			}
			return false
		})
		if found {
			return d
		}
	}
	return maxSearchRadius
}

// NearestFoodDir returns the direction (1=N,2=E,3=S,4=W) toward nearest food, or 0.
func (w *World) NearestFoodDir(x, y int) int {
	for d := 0; d <= maxSearchRadius; d++ {
		bx, by := -1, -1
		w.scanManhattanRing(x, y, d, func(fx, fy int) bool {
			if w.TileAt(fx, fy).Type() == TileFood {
				bx, by = fx, fy
				return true
			}
			return false
		})
		if bx >= 0 {
			return directionToward(x, y, bx, by)
		}
	}
	return DirNone
}

// NearestNPC returns Manhattan distance to nearest other NPC, or 31 if none.
func (w *World) NearestNPC(x, y int, excludeID uint16) int {
	for d := 1; d <= maxSearchRadius; d++ {
		found := false
		w.scanManhattanRing(x, y, d, func(fx, fy int) bool {
			occ := w.OccAt(fx, fy)
			if occ != 0 && occ != excludeID {
				if npc := w.npcByID[occ]; npc != nil && npc.Alive() {
					found = true
					return true
				}
			}
			return false
		})
		if found {
			return d
		}
	}
	return maxSearchRadius
}

// NearestNPCID returns the ID of the nearest other NPC, or 0 if none.
func (w *World) NearestNPCID(x, y int, excludeID uint16) uint16 {
	for d := 1; d <= maxSearchRadius; d++ {
		bestID := uint16(0)
		w.scanManhattanRing(x, y, d, func(fx, fy int) bool {
			occ := w.OccAt(fx, fy)
			if occ != 0 && occ != excludeID {
				if npc := w.npcByID[occ]; npc != nil && npc.Alive() {
					bestID = occ
					return true
				}
			}
			return false
		})
		if bestID != 0 {
			return bestID
		}
	}
	return 0
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

// NearestNPCFull returns (distance, ID, direction) to nearest other NPC in a single scan.
func (w *World) NearestNPCFull(x, y int, excludeID uint16) (int, uint16, int) {
	for d := 1; d <= maxSearchRadius; d++ {
		bestID := uint16(0)
		bx, by := -1, -1
		w.scanManhattanRing(x, y, d, func(fx, fy int) bool {
			occ := w.OccAt(fx, fy)
			if occ != 0 && occ != excludeID {
				if npc := w.npcByID[occ]; npc != nil && npc.Alive() {
					bestID = occ
					bx, by = fx, fy
					return true
				}
			}
			return false
		})
		if bestID != 0 {
			return d, bestID, directionToward(x, y, bx, by)
		}
	}
	return maxSearchRadius, 0, DirNone
}

// NearestNPCDir returns the direction toward the nearest other NPC, or 0.
func (w *World) NearestNPCDir(x, y int, excludeID uint16) int {
	for d := 1; d <= maxSearchRadius; d++ {
		bx, by := -1, -1
		w.scanManhattanRing(x, y, d, func(fx, fy int) bool {
			occ := w.OccAt(fx, fy)
			if occ != 0 && occ != excludeID {
				if npc := w.npcByID[occ]; npc != nil && npc.Alive() {
					bx, by = fx, fy
					return true
				}
			}
			return false
		})
		if bx >= 0 {
			return directionToward(x, y, bx, by)
		}
	}
	return DirNone
}

// NearestItemDir returns the direction toward the nearest item tile, or 0.
func (w *World) NearestItemDir(x, y int) int {
	for d := 0; d <= maxSearchRadius; d++ {
		bx, by := -1, -1
		w.scanManhattanRing(x, y, d, func(fx, fy int) bool {
			if isItem(w.TileAt(fx, fy).Type()) {
				bx, by = fx, fy
				return true
			}
			return false
		})
		if bx >= 0 {
			return directionToward(x, y, bx, by)
		}
	}
	return DirNone
}

// ItemCount returns the number of item tiles (tool, weapon, treasure, crystal) on the map.
func (w *World) ItemCount() int {
	return w.itemCount
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
		if w.TileAt(x, y).Type() == TileEmpty && w.OccAt(x, y) == 0 {
			if w.Rng.Intn(10) == 0 {
				w.SetTile(x, y, MakeTile(TilePoison))
				w.PoisonTTL[w.idx(x, y)] = w.Tick
			} else {
				var itemType byte
				if w.Rng.Intn(20) == 0 {
					itemType = TileCrystal // 1-in-20 chance
				} else {
					itemType = byte(TileTool + w.Rng.Intn(3))
				}
				w.SetTile(x, y, MakeTile(itemType))
			}
			break
		}
	}
}

// NearestItem returns (Manhattan distance, tile type) of nearest item tile, or (31, 0) if none.
func (w *World) NearestItem(x, y int) (int, byte) {
	for d := 0; d <= maxSearchRadius; d++ {
		bestType := byte(0)
		w.scanManhattanRing(x, y, d, func(fx, fy int) bool {
			typ := w.TileAt(fx, fy).Type()
			if isItem(typ) {
				bestType = typ
				return true
			}
			return false
		})
		if bestType != 0 {
			return d, bestType
		}
	}
	return maxSearchRadius, 0
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
	for d := 0; d <= maxSearchRadius; d++ {
		found := false
		w.scanManhattanRing(x, y, d, func(fx, fy int) bool {
			if w.TileAt(fx, fy).Type() == TilePoison {
				found = true
				return true
			}
			return false
		})
		if found {
			return d
		}
	}
	return maxSearchRadius
}

// DecayPoison removes poison tiles that have existed for >= 200 ticks.
func (w *World) DecayPoison() {
	for idx, placedTick := range w.PoisonTTL {
		if w.Tick-placedTick >= 200 {
			y := idx / w.Size
			x := idx % w.Size
			if w.TileAt(x, y).Type() == TilePoison {
				w.SetTile(x, y, MakeTile(TileEmpty))
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
					w.SetTile(x, y, MakeTile(TileEmpty))
				}
			}
		}
	}
}

// AutoWorldSize returns an appropriate world size for the given number of NPCs.
func AutoWorldSize(npcs int) int {
	s := int(math.Sqrt(float64(npcs))) * 4
	if s < 32 {
		s = 32
	}
	return s
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
