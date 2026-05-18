// Pricing tables + cost calc for Claude usage telemetry.
//
// Rates frozen at 2026-04-29; verify against
// console.anthropic.com/billing before relying on absolute USD figures.
// Drift risk: Anthropic adjusts model rates and ships new model IDs
// without a stable schema mapping — keep this table best-effort and
// treat unknown models as "no cost" (return ok=false), never zero.

package claude

// modelRate captures input + output USD per 1M tokens.
type modelRate struct {
	inputPerM  float64
	outputPerM float64
}

// modelRates is the per-model rate table. Values from Anthropic
// pricing page as of 2026-04-29.
var modelRates = map[string]modelRate{
	"claude-opus-4-7":           {inputPerM: 15.00, outputPerM: 75.00},
	"claude-sonnet-4-6":         {inputPerM: 3.00, outputPerM: 15.00},
	"claude-haiku-4-5-20251001": {inputPerM: 1.00, outputPerM: 5.00},
}

// costUSD computes total session cost in USD given token counts.
//
// Cache pricing per Anthropic prompt-caching docs (2026-04-29):
//   - cache reads bill at 0.1× the model's input rate
//   - cache writes bill at 1.25× the model's input rate
//
// Returns (cost, ok). ok=false when model is not in the rate table —
// callers should skip writing usage.cost_usd in that case rather than
// recording zero.
func costUSD(
	model string,
	inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens int,
) (float64, bool) {
	r, ok := modelRates[model]
	if !ok {
		return 0, false
	}
	const perM = 1_000_000.0
	cost := float64(inputTokens)*r.inputPerM/perM +
		float64(outputTokens)*r.outputPerM/perM +
		float64(cacheReadTokens)*r.inputPerM*0.1/perM +
		float64(cacheWriteTokens)*r.inputPerM*1.25/perM
	return cost, true
}
