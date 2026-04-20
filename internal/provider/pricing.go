// IMPORTANT DISCLAIMER:
// The pricing data in this file is for ESTIMATION purposes only and may NOT be accurate.
// - Prices are subject to change by providers without notice
// - Different regions may have different pricing
// - Volume discounts, enterprise pricing, and special offers are NOT reflected
// - Some models may have different pricing for different context lengths
// - Token counting methods may vary between providers//
// These values are PLACEHOLDERS to give users a rough cost estimate.
// For accurate billing information, always refer to the official provider documentation:
//   - OpenAI: https://openai.com/pricing
//   - Anthropic: https://www.anthropic.com/pricing
//   - Google Gemini: https://cloud.google.com/vertex-ai/generative-ai/pricing
//
// Users should treat these estimates as approximate and verify actual costs
// through their provider dashboards.
package provider

// ModelPricing defines token pricing for a specific model
type ModelPricing struct {
	InputPricePer1K  float64 // USD per 1,000 input tokens
	OutputPricePer1K float64 // USD per 1,000 output tokens
}

// GetPricing returns the pricing for a given provider and model
func GetPricing(provider, model string) ModelPricing {
	pricing := map[string]map[string]ModelPricing{
		"openai": {
			"gpt-4o":        {InputPricePer1K: 0.0025, OutputPricePer1K: 0.0100},
			"gpt-4o-mini":   {InputPricePer1K: 0.00015, OutputPricePer1K: 0.00060},
			"gpt-4-turbo":   {InputPricePer1K: 0.0100, OutputPricePer1K: 0.0300},
			"gpt-3.5-turbo": {InputPricePer1K: 0.0005, OutputPricePer1K: 0.0015},
		},
		"anthropic": {
			"claude-4-5-sonnet": {InputPricePer1K: 0.0030, OutputPricePer1K: 0.0150},
			"claude-4-5-opus":   {InputPricePer1K: 0.0150, OutputPricePer1K: 0.0750},
			"claude-4-5-haiku":  {InputPricePer1K: 0.00025, OutputPricePer1K: 0.00125},
		},
		"gemini": {
			"gemini-flash-2.5": {InputPricePer1K: 0.00010, OutputPricePer1K: 0.00040},
			"gemini-pro-2.5":   {InputPricePer1K: 0.00125, OutputPricePer1K: 0.00500},
		},
		"ollama": {
			"default": {InputPricePer1K: 0.0, OutputPricePer1K: 0.0},
		},
	}

	if providerPricing, ok := pricing[provider]; ok {
		if modelPricing, ok := providerPricing[model]; ok {
			return modelPricing
		}
		// Return default for provider if model not found
		for _, p := range providerPricing {
			return p
		}
	}

	return ModelPricing{InputPricePer1K: 0.0, OutputPricePer1K: 0.0}
}

// CalculateCost calculates the cost based on token usage and pricing
func CalculateCost(promptTokens, completionTokens int, pricing ModelPricing) float64 {
	inputCost := (float64(promptTokens) / 1000.0) * pricing.InputPricePer1K
	outputCost := (float64(completionTokens) / 1000.0) * pricing.OutputPricePer1K
	return inputCost + outputCost
}
