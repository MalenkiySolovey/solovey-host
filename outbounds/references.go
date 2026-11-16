package outbounds

import (
	"encoding/json"
	"fmt"
)

func IsLimiter(kind string) bool {
	return kind == "bandwidth-limiter" || kind == "connection-limiter" || kind == "rate-limiter" || kind == "traffic-limiter"
}

// LimiterReferences retains the existing Routing tag semantics inside a native
// limiter. It observes references without rewriting the stored rules/options.
func LimiterReferences(raw json.RawMessage) ([]string, error) {
	var fields struct {
		Route struct {
			Final   string           `json:"final"`
			Rules   []map[string]any `json:"rules"`
			RuleSet []struct {
				DownloadDetour string `json:"download_detour"`
			} `json:"rule_set"`
		} `json:"route"`
	}
	if json.Unmarshal(raw, &fields) != nil {
		return nil, fmt.Errorf("OUTBOUND_LIMITER_ROUTE_INVALID")
	}
	var refs []string
	seen := map[string]bool{}
	add := func(tag string) {
		if tag != "" && !seen[tag] {
			refs = append(refs, tag)
			seen[tag] = true
		}
	}
	add(fields.Route.Final)
	var visit func(map[string]any)
	visit = func(rule map[string]any) {
		if tag, ok := rule["outbound"].(string); ok {
			add(tag)
		}
		if rules, ok := rule["rules"].([]any); ok {
			for _, item := range rules {
				if child, ok := item.(map[string]any); ok {
					visit(child)
				}
			}
		}
	}
	for _, rule := range fields.Route.Rules {
		visit(rule)
	}
	for _, ruleSet := range fields.Route.RuleSet {
		add(ruleSet.DownloadDetour)
	}
	return refs, nil
}
