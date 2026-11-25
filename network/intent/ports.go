package intent

import (
	"cmp"
	"errors"
	"slices"
)

// PortInterval is an inclusive, positive TCP or UDP port range.
type PortInterval struct {
	First uint16 `json:"first"`
	Last  uint16 `json:"last"`
}

// CanonicalPortIntervals forms a sorted, disjoint union without expanding a
// range into individual ports. Adjacent intervals share one representation.
func CanonicalPortIntervals(ports []uint16, intervals ...PortInterval) ([]PortInterval, error) {
	if len(ports) == 0 && len(intervals) == 0 {
		return nil, nil
	}
	all := make([]PortInterval, 0, len(ports)+len(intervals))
	all = append(all, intervals...)
	for _, port := range ports {
		all = append(all, PortInterval{First: port, Last: port})
	}
	slices.SortFunc(all, func(a, b PortInterval) int { return cmp.Compare(a.First, b.First) })
	result := all[:0]
	for _, next := range all {
		if next.First == 0 || next.Last < next.First {
			return nil, errors.New("port interval must be an ascending, positive inclusive range")
		}
		if i := len(result) - 1; i >= 0 && uint32(next.First) <= uint32(result[i].Last)+1 {
			result[i].Last = max(result[i].Last, next.Last)
			continue
		}
		result = append(result, next)
	}
	return result, nil
}
