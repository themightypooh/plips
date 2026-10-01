package sim

import (
	"math/rand"
	"testing"
)

func TestRemovePlipKeepsOwnershipConsistent(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	w := NewWorld(RoomW, RoomH, RoomFloor, 1)
	for i := 0; i < 3; i++ {
		w.AddPlip(NewPlip(RandomGenome(r), RandomName(r), true), float32(60+i*80), nil)
	}
	w.Splash(100, 40, 2, 30)
	for i := 0; i < 300; i++ {
		w.Step()
	}
	keep := w.Plips[2]
	before := keep.Mass
	w.RemovePlip(w.Plips[0])
	if len(w.Plips) != 2 || keep.ID != 2 {
		t.Fatalf("ids not renumbered: %d plips, keep.ID=%d", len(w.Plips), keep.ID)
	}
	for i := 0; i < w.N; i++ {
		if int(w.Own[i]) > len(w.Plips) {
			t.Fatalf("particle %d owned by missing plip %d", i, w.Own[i])
		}
	}
	for i := 0; i < 300; i++ {
		w.Step()
	}
	if keep.Mass < before/2 {
		t.Fatalf("kept plip lost its body: %d -> %d", before, keep.Mass)
	}
	w.ClearLoose()
	for i := 0; i < w.N; i++ {
		if w.Own[i] == 0 {
			t.Fatal("loose particle survived ClearLoose")
		}
	}
}
