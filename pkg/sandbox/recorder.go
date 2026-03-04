package sandbox

import (
	"bufio"
	"encoding/json"
	"os"
)

// RecordHeader is the first line of a JSONL recording.
type RecordHeader struct {
	Type      string `json:"type"`       // "header"
	Seed      int64  `json:"seed"`
	NPCs      int    `json:"npcs"`
	WorldSize int    `json:"world_size"`
	Ticks     int    `json:"ticks"`
	EveryN    int    `json:"every_n"`
	Biomes    bool   `json:"biomes"`
	BiomeGrid []byte `json:"biome_grid"` // base64 via json.Marshal
}

// RecordNPC captures per-NPC state in a tick frame.
type RecordNPC struct {
	ID     uint16 `json:"id"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	HP     int    `json:"hp"`
	Energy int    `json:"e"`
	Item   byte   `json:"it"`
	Gold   int    `json:"g"`
	Age    int    `json:"a"`
	Fit    int    `json:"f"`
	Stress int    `json:"s"`
	GenLen int    `json:"gl"`
}

// RecordFullNPC includes genome bytes (used in full frames).
type RecordFullNPC struct {
	RecordNPC
	Genome []byte `json:"gen"`
}

// RecordStats captures cumulative scheduler counters.
type RecordStats struct {
	Attacks    int `json:"atk"`
	Kills      int `json:"kil"`
	Heals      int `json:"hea"`
	Harvests   int `json:"har"`
	Terraforms int `json:"ter"`
	Trades     int `json:"trd"`
	Teaches    int `json:"tch"`
}

// RecordFrame is a tick snapshot (type="tick").
type RecordFrame struct {
	Type     string      `json:"type"` // "tick" or "full"
	Tick     int         `json:"t"`
	NPCs     []RecordNPC `json:"npcs"`
	Grid     []byte      `json:"grid"` // tile bytes, base64 via json
	Stats    RecordStats `json:"s"`
	FoodRate float64     `json:"fr"`
}

// RecordFullFrame is a full snapshot with genome data (type="full").
type RecordFullFrame struct {
	Type     string          `json:"type"` // "full"
	Tick     int             `json:"t"`
	NPCs     []RecordFullNPC `json:"npcs"`
	Grid     []byte          `json:"grid"`
	Stats    RecordStats     `json:"s"`
	FoodRate float64         `json:"fr"`
}

// Recorder writes simulation snapshots to a JSONL file.
type Recorder struct {
	everyN int
	w      *bufio.Writer
	f      *os.File
	enc    *json.Encoder
}

// NewRecorder creates a recorder writing to the given path, snapshotting every everyN ticks.
func NewRecorder(path string, everyN int) (*Recorder, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	bw := bufio.NewWriter(f)
	return &Recorder{
		everyN: everyN,
		w:      bw,
		f:      f,
		enc:    json.NewEncoder(bw),
	}, nil
}

// WriteHeader writes the header line (call once before tick loop).
func (r *Recorder) WriteHeader(h RecordHeader) error {
	h.Type = "header"
	return r.enc.Encode(h)
}

// RecordTick snapshots the world if tick is aligned to everyN.
func (r *Recorder) RecordTick(tick int, w *World, s *Scheduler) error {
	if tick%r.everyN != 0 {
		return nil
	}

	stats := RecordStats{
		Attacks:    s.AttackCount,
		Kills:      s.KillCount,
		Heals:      s.HealCount,
		Harvests:   s.HarvestCount,
		Terraforms: s.TerraformCount,
		Trades:     s.TradeCount,
		Teaches:    s.TeachCount,
	}

	// Extract grid as raw bytes
	grid := make([]byte, len(w.Grid))
	for i, t := range w.Grid {
		grid[i] = byte(t)
	}

	// Full frame every 10×everyN
	if tick%(r.everyN*10) == 0 {
		npcs := make([]RecordFullNPC, 0, len(w.NPCs))
		for _, npc := range w.NPCs {
			if !npc.Alive() {
				continue
			}
			gen := make([]byte, len(npc.Genome))
			copy(gen, npc.Genome)
			npcs = append(npcs, RecordFullNPC{
				RecordNPC: makeRecordNPC(npc),
				Genome:    gen,
			})
		}
		return r.enc.Encode(RecordFullFrame{
			Type:     "full",
			Tick:     tick,
			NPCs:     npcs,
			Grid:     grid,
			Stats:    stats,
			FoodRate: w.FoodRate,
		})
	}

	// Regular tick frame
	npcs := make([]RecordNPC, 0, len(w.NPCs))
	for _, npc := range w.NPCs {
		if !npc.Alive() {
			continue
		}
		npcs = append(npcs, makeRecordNPC(npc))
	}
	return r.enc.Encode(RecordFrame{
		Type:     "tick",
		Tick:     tick,
		NPCs:     npcs,
		Grid:     grid,
		Stats:    stats,
		FoodRate: w.FoodRate,
	})
}

func makeRecordNPC(npc *NPC) RecordNPC {
	return RecordNPC{
		ID:     npc.ID,
		X:      npc.X,
		Y:      npc.Y,
		HP:     npc.Health,
		Energy: npc.Energy,
		Item:   npc.Item,
		Gold:   npc.Gold,
		Age:    npc.Age,
		Fit:    npc.Fitness,
		Stress: npc.Stress,
		GenLen: len(npc.Genome),
	}
}

// Close flushes and closes the recording file.
func (r *Recorder) Close() error {
	if err := r.w.Flush(); err != nil {
		r.f.Close()
		return err
	}
	return r.f.Close()
}
