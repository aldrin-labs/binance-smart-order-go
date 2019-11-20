package testing

import "testing"


func TestStartingTrailingForEntry(t *testing.T) {
	var v float64
	v = 1.6
	if v != 1.5 {
		t.Error("Expected 1.5, got ", v)
	}
}
func TestStopLossOnTimeout(t *testing.T) {
	var v float64
	v = 1.6
	if v != 1.5 {
		t.Error("Expected 1.5, got ", v)
	}
}

func TestStopLossForceExit(t *testing.T) {
	var v float64
	v = 1.6
	if v != 1.5 {
		t.Error("Expected 1.5, got ", v)
	}
}

func TestReachingOneOfTwoProfitTargetsAndStopLoss(t *testing.T) {
	var v float64
	v = 1.6
	if v != 1.5 {
		t.Error("Expected 1.5, got ", v)
	}
}

func TestReachingThreeOfFiveProfitTargets(t *testing.T) {
	var v float64
	v = 1.6
	if v != 1.5 {
		t.Error("Expected 1.5, got ", v)
	}
}

func TestCanceling(t *testing.T) {
	var v float64
	v = 1.6
	if v != 1.5 {
		t.Error("Expected 1.5, got ", v)
	}
}