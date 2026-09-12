package prices

import "testing"

// The sign convention is the whole point of the column: a symbol trading below
// its sector aggregate is cheap, and the UI colours it green off this sign. Get
// it backwards and every valuation reading on both tables inverts.
func TestVsSectorSignsCheapNegative(t *testing.T) {
	cheap := 30.0
	dear := 60.0
	sectorPE := 40.0

	got := vsSector(&cheap, sectorPE)
	if got == nil || *got >= 0 {
		t.Fatalf("P/E below the sector aggregate must read negative, got %v", got)
	}
	if want := -25.0; *got != want {
		t.Errorf("want %v%%, got %v%%", want, *got)
	}

	got = vsSector(&dear, sectorPE)
	if got == nil || *got <= 0 {
		t.Fatalf("P/E above the sector aggregate must read positive, got %v", got)
	}
}

// A missing or nonsensical input must produce no comparison at all rather than a
// number: the tables render nil as "—", and a fabricated 0% would read as
// "exactly in line with its sector", which is a claim we cannot make.
func TestVsSectorRefusesUnusableInput(t *testing.T) {
	pe := 20.0
	negative := -5.0

	cases := map[string]struct {
		own    *float64
		sector float64
	}{
		"no company figure":   {nil, 40},
		"no sector aggregate": {&pe, 0},
		"negative earnings":   {&negative, 40},
	}
	for name, c := range cases {
		if got := vsSector(c.own, c.sector); got != nil {
			t.Errorf("%s: want no comparison, got %v", name, *got)
		}
	}
}

// A sector Yahoo has no usable peer data for is cached as a negative result.
// Without that, every symbol in the sector retries the aggregate — a dozen
// upstream calls each — on every single list refresh.
func TestSectorNegativeResultIsCached(t *testing.T) {
	sectorCache.Delete("Nowhere")
	storeSector("Nowhere", nil)

	val, ok := cachedSector("Nowhere")
	if !ok {
		t.Fatal("negative result must be cached, not treated as a miss")
	}
	if val != nil {
		t.Errorf("want nil valuation, got %+v", val)
	}
}
