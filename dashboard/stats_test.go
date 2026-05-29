package dashboard

import (
	"math"
	"testing"
	"time"

	"dexmon/types"
)

func makeReadings(vals ...int) []types.Reading {
	t := time.Now().UTC()
	out := make([]types.Reading, len(vals))
	for i, v := range vals {
		out[i] = types.Reading{Value: v, RecordedAt: t}
	}
	return out
}

func TestComputeStats_Empty(t *testing.T) {
	s := computeStats(nil, 70, 180)
	if s != (StatsJSON{}) {
		t.Errorf("expected zero StatsJSON for nil input, got %+v", s)
	}
}

func TestComputeStats_Single(t *testing.T) {
	s := computeStats(makeReadings(100), 70, 180)
	if s.High != 100 { t.Errorf("High: got %d want 100", s.High) }
	if s.Low != 100  { t.Errorf("Low: got %d want 100", s.Low) }
	if s.Avg != 100  { t.Errorf("Avg: got %d want 100", s.Avg) }
	if s.StdDev != 0 { t.Errorf("StdDev: got %d want 0", s.StdDev) }
	if s.CV != 0     { t.Errorf("CV: got %v want 0", s.CV) }
	if s.TimeInRange != 100.0 {
		t.Errorf("TimeInRange: got %v want 100.0", s.TimeInRange)
	}
	if s.TimeBelowRange != 0.0 {
		t.Errorf("TimeBelowRange: got %v want 0.0", s.TimeBelowRange)
	}
	if s.TimeAboveRange != 0.0 {
		t.Errorf("TimeAboveRange: got %v want 0.0", s.TimeAboveRange)
	}
	if s.Q1 != 100 || s.Median != 100 || s.Q3 != 100 {
		t.Errorf("Quartiles: got Q1=%d Median=%d Q3=%d, want all 100", s.Q1, s.Median, s.Q3)
	}
}

// Dataset: [70,80,100,120,150,200], target 70–180
// mean=120, stddev≈44, CV≈37.0, TIR=5/6≈83.3%, TBR=0%, TAR=1/6≈16.7%
// sorted: Q1=sorted[1]=80, Median=sorted[2]=100, Q3=sorted[3]=120
func TestComputeStats_KnownDataset(t *testing.T) {
	s := computeStats(makeReadings(70, 80, 100, 120, 150, 200), 70, 180)

	if s.High != 200 { t.Errorf("High: got %d want 200", s.High) }
	if s.Low != 70   { t.Errorf("Low: got %d want 70", s.Low) }
	if s.Avg != 120  { t.Errorf("Avg: got %d want 120", s.Avg) }
	if s.StdDev != 44 { t.Errorf("StdDev: got %d want 44", s.StdDev) }
	if math.Abs(s.CV-37.0) > 0.15 {
		t.Errorf("CV: got %v want ~37.0", s.CV)
	}
	if math.Abs(s.TimeInRange-83.3) > 0.15 {
		t.Errorf("TimeInRange: got %v want ~83.3", s.TimeInRange)
	}
	if s.TimeBelowRange != 0.0 {
		t.Errorf("TimeBelowRange: got %v want 0.0", s.TimeBelowRange)
	}
	if math.Abs(s.TimeAboveRange-16.7) > 0.15 {
		t.Errorf("TimeAboveRange: got %v want ~16.7", s.TimeAboveRange)
	}
	if s.Q1 != 80     { t.Errorf("Q1: got %d want 80", s.Q1) }
	if s.Median != 100 { t.Errorf("Median: got %d want 100", s.Median) }
	if s.Q3 != 120    { t.Errorf("Q3: got %d want 120", s.Q3) }
}

func TestComputeStats_AllOutOfRange(t *testing.T) {
	// All below range: TIR=0, TBR=100, TAR=0
	s := computeStats(makeReadings(50, 55, 60), 70, 180)
	if s.TimeInRange != 0.0 {
		t.Errorf("TimeInRange: got %v want 0.0", s.TimeInRange)
	}
	if s.TimeBelowRange != 100.0 {
		t.Errorf("TimeBelowRange: got %v want 100.0", s.TimeBelowRange)
	}
	if s.TimeAboveRange != 0.0 {
		t.Errorf("TimeAboveRange: got %v want 0.0", s.TimeAboveRange)
	}
}

func TestComputeStats_CVZeroMeanGuard(t *testing.T) {
	// Can't construct zero-mean glucose readings in practice,
	// but verify zero-mean readings return CV=0 not a panic.
	readings := []types.Reading{{Value: 0}, {Value: 0}}
	s := computeStats(readings, 70, 180)
	if s.CV != 0 {
		t.Errorf("CV with zero mean: got %v want 0", s.CV)
	}
}
