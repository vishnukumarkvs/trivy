package ignore

import (
	"slices"
	"time"

	"github.com/samber/lo"

	"github.com/aquasecurity/trivy/pkg/iac/types"
)

// Ignorer represents a function that checks if the rule should be ignored.
type Ignorer func(resultMeta types.Metadata, param any) bool

type Rules []Rule

// Ignore checks if the rule should be ignored based on provided metadata, IDs, and ignorer functions.
func (r Rules) Ignore(m types.Metadata, ids []string, ignorers map[string]Ignorer) bool {
	return slices.ContainsFunc(r, func(r Rule) bool {
		return r.ignore(m, ids, ignorers)
	})
}

func (r Rules) shift() {
	var (
		currentRange *types.Range
		offset       int
	)

	for i := len(r) - 1; i > 0; i-- {
		currentIgnore, nextIgnore := r[i], r[i-1]
		if currentRange == nil {
			currentRange = &currentIgnore.rng
		}
		if nextIgnore.rng.GetStartLine()+1+offset == currentIgnore.rng.GetStartLine() {
			r[i-1].rng = *currentRange
			offset++
		} else {
			currentRange = nil
			offset = 0
		}
	}
}

// Rule represents a rule for ignoring vulnerabilities.
type Rule struct {
	rng      types.Range
	sections map[string]any
}

func (r Rule) ignore(m types.Metadata, ids []string, ignorers map[string]Ignorer) bool {
	matchMeta, ok := r.matchRange(&m)
	if !ok {
		return false
	}

	ignorers = lo.Assign(defaultIgnorers(ids), ignorers)

	for ignoreID, ignore := range ignorers {
		if param, exists := r.sections[ignoreID]; exists {
			if !ignore(*matchMeta, param) {
				return false
			}
		}
	}

	return true
}

func (r Rule) matchRange(m *types.Metadata) (*types.Metadata, bool) {
	metaHierarchy := m
	for metaHierarchy != nil {
		if r.rng.GetFilename() != metaHierarchy.Range().GetFilename() {
			metaHierarchy = metaHierarchy.Parent()
			continue
		}
		if metaHierarchy.Range().GetStartLine() == r.rng.GetStartLine()+1 ||
			metaHierarchy.Range().GetStartLine() == r.rng.GetStartLine() {
			return metaHierarchy, true
		}
		metaHierarchy = metaHierarchy.Parent()
	}

	return nil, false
}

func defaultIgnorers(ids []string) map[string]Ignorer {
	return map[string]Ignorer{
		"id": func(_ types.Metadata, param any) bool {
			id, ok := param.(string)
			return ok && (id == "*" || len(ids) == 0 || slices.Contains(ids, id))
		},
		"exp": func(_ types.Metadata, param any) bool {
			expiry, ok := param.(time.Time)
			return ok && time.Now().Before(expiry)
		},
	}
}
