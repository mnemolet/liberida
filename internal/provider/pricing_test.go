package provider

import (
	"testing"
)

func TestGetPricing(t *testing.T) {
	tests := []struct {
		provider string
		model    string
		wantCost bool // whether cost should be > 0
	}{
		{"ollama", "llama2", false},
		{"unknown", "unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.provider+"-"+tt.model, func(t *testing.T) {
			pricing := GetPricing(tt.provider, tt.model)
			if tt.wantCost && pricing.InputPricePer1K == 0 && pricing.OutputPricePer1K == 0 {
				t.Error("Expected non-zero pricing, got zero")
			}
			if !tt.wantCost && (pricing.InputPricePer1K != 0 || pricing.OutputPricePer1K != 0) {
				t.Error("Expected zero pricing, got non-zero")
			}
		})
	}
}

func TestCalculateCost(t *testing.T) {
	pricing := ModelPricing{InputPricePer1K: 0.0025, OutputPricePer1K: 0.0100}

	cost := CalculateCost(1000, 500, pricing)
	expected := 0.0025 + 0.0050 // $0.0025 + $0.0050 = $0.0075

	if cost != expected {
		t.Errorf("Expected cost %.6f, got %.6f", expected, cost)
	}
}
