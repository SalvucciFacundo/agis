package memory

import (
	"math"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/core"
)

func TestAggregateUserModel_FirstWrite(t *testing.T) {
	obs := []core.Observation{
		{TopicKey: "user/pref/coffee", Content: "dark roast", Importance: 4},
	}
	got := AggregateUserModel(nil, obs)

	if len(got) != 1 {
		t.Fatalf("got %d rows, want 1", len(got))
	}
	if got[0].Key != "user/pref/coffee" {
		t.Errorf("key = %q, want %q", got[0].Key, "user/pref/coffee")
	}
	if got[0].Value != "dark roast" {
		t.Errorf("value = %q, want %q", got[0].Value, "dark roast")
	}
	if got[0].Confidence != 0.8 {
		t.Errorf("confidence = %v, want 0.8 (4/5)", got[0].Confidence)
	}
}

func TestAggregateUserModel_UpdateBlend(t *testing.T) {
	existing := []core.UserModel{
		{Key: "user/pref/coffee", Value: "dark roast", Confidence: 0.8},
	}
	obs := []core.Observation{
		{TopicKey: "user/pref/coffee", Content: "medium roast", Importance: 3},
	}
	got := AggregateUserModel(existing, obs)

	if len(got) != 1 {
		t.Fatalf("got %d rows, want 1", len(got))
	}
	// 0.7*0.8 + 0.3*0.6 = 0.56 + 0.18 = 0.74.
	if got[0].Confidence != 0.74 {
		t.Errorf("confidence = %v, want 0.74", got[0].Confidence)
	}
	if got[0].Value != "medium roast" {
		t.Errorf("value = %q, want %q (latest content wins)", got[0].Value, "medium roast")
	}
}

func TestAggregateUserModel_NonUserExcluded(t *testing.T) {
	obs := []core.Observation{
		{TopicKey: "project/arch", Content: "hexagonal", Importance: 5},
		{TopicKey: "user/pref/coffee", Content: "dark roast", Importance: 4},
	}
	got := AggregateUserModel(nil, obs)

	if len(got) != 1 {
		t.Fatalf("got %d rows, want 1 (project/* excluded)", len(got))
	}
	if got[0].Key != "user/pref/coffee" {
		t.Errorf("key = %q, want %q", got[0].Key, "user/pref/coffee")
	}
}

func TestAggregateUserModel_ConfidenceClamped(t *testing.T) {
	obs := []core.Observation{
		{TopicKey: "user/too-low", Content: "a", Importance: -10}, // -2 → clamp 0
		{TopicKey: "user/too-high", Content: "b", Importance: 100}, // 20 → clamp 1
	}
	got := AggregateUserModel(nil, obs)

	if len(got) != 2 {
		t.Fatalf("got %d rows, want 2", len(got))
	}
	byKey := map[string]core.UserModel{}
	for _, u := range got {
		byKey[u.Key] = u
	}
	if byKey["user/too-low"].Confidence != 0 {
		t.Errorf("user/too-low confidence = %v, want 0", byKey["user/too-low"].Confidence)
	}
	if byKey["user/too-high"].Confidence != 1 {
		t.Errorf("user/too-high confidence = %v, want 1", byKey["user/too-high"].Confidence)
	}
}

func TestAggregateUserModel_ExistingPreserved(t *testing.T) {
	existing := []core.UserModel{
		{Key: "user/pref/coffee", Value: "dark roast", Confidence: 0.9},
		{Key: "user/pref/tea", Value: "green", Confidence: 0.5},
	}
	// Only coffee is touched by new observations; tea must be preserved.
	obs := []core.Observation{
		{TopicKey: "user/pref/coffee", Content: "light roast", Importance: 2},
	}
	got := AggregateUserModel(existing, obs)

	if len(got) != 2 {
		t.Fatalf("got %d rows, want 2", len(got))
	}
	byKey := map[string]core.UserModel{}
	for _, u := range got {
		byKey[u.Key] = u
	}
	if tea, ok := byKey["user/pref/tea"]; !ok || tea.Confidence != 0.5 {
		t.Errorf("user/pref/tea not preserved: %+v", tea)
	}
	// coffee: 0.7*0.9 + 0.3*0.4 = 0.63 + 0.12 = 0.75.
	want := 0.7*0.9 + 0.3*0.4
	if byKey["user/pref/coffee"].Confidence != want {
		t.Errorf("coffee confidence = %v, want %v", byKey["user/pref/coffee"].Confidence, want)
	}
}

func TestAggregateUserModel_EmptyInput(t *testing.T) {
	if got := AggregateUserModel(nil, nil); len(got) != 0 {
		t.Errorf("got %d rows, want 0", len(got))
	}
}

func TestClamp01(t *testing.T) {
	tests := []struct {
		in   float64
		want float64
	}{
		{-1, 0},
		{0, 0},
		{0.5, 0.5},
		{1, 1},
		{2, 1},
		{math.Inf(1), 1},
		{math.Inf(-1), 0},
	}
	for _, tt := range tests {
		if got := clamp01(tt.in); got != tt.want {
			t.Errorf("clamp01(%v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
