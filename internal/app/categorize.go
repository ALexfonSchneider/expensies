package app

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

// Categorizer applies category rules to transactions. Rules are compiled
// once per instance because an import evaluates them for every line.
type Categorizer struct {
	rules []compiledRule
}

type compiledRule struct {
	rule  domain.Rule
	value string
	re    *regexp.Regexp
}

// NewCategorizer validates and compiles rules. Problems are reported as
// domain.ErrInvalid so the API can reject a rule when it is created rather
// than at the next import.
func NewCategorizer(rules []domain.Rule) (*Categorizer, error) {
	compiled := make([]compiledRule, 0, len(rules))
	for _, r := range rules {
		if err := ValidateRule(r); err != nil {
			return nil, err
		}
		cr := compiledRule{rule: r, value: strings.ToLower(strings.TrimSpace(r.Value))}
		if r.Op == domain.RuleOpRegex {
			re, err := regexp.Compile("(?i)" + r.Value)
			if err != nil {
				return nil, fmt.Errorf("%w: rule %d: %v", domain.ErrInvalid, r.ID, err)
			}
			cr.re = re
		}
		compiled = append(compiled, cr)
	}
	sort.SliceStable(compiled, func(i, j int) bool {
		a, b := compiled[i].rule, compiled[j].rule
		if a.Priority != b.Priority {
			return a.Priority < b.Priority
		}
		return a.ID < b.ID
	})
	return &Categorizer{rules: compiled}, nil
}

// ValidateRule checks field, operator and pattern syntax.
func ValidateRule(r domain.Rule) error {
	switch r.Field {
	case domain.RuleFieldMerchant, domain.RuleFieldDescription, domain.RuleFieldMCC,
		domain.RuleFieldKind, domain.RuleFieldCounterparty:
	default:
		return fmt.Errorf("%w: unknown rule field %q", domain.ErrInvalid, r.Field)
	}
	switch r.Op {
	case domain.RuleOpContains, domain.RuleOpEquals, domain.RuleOpPrefix:
	case domain.RuleOpRegex:
		if _, err := regexp.Compile("(?i)" + r.Value); err != nil {
			return fmt.Errorf("%w: bad regular expression: %v", domain.ErrInvalid, err)
		}
	default:
		return fmt.Errorf("%w: unknown rule operator %q", domain.ErrInvalid, r.Op)
	}
	if strings.TrimSpace(r.Value) == "" {
		return fmt.Errorf("%w: rule value is empty", domain.ErrInvalid)
	}
	if r.CategoryID == 0 {
		return fmt.Errorf("%w: rule has no category", domain.ErrInvalid)
	}
	return nil
}

// Match returns the category of the first rule that matches tx.
func (c *Categorizer) Match(tx *domain.Transaction) (int64, bool) {
	for _, cr := range c.rules {
		if cr.matches(tx) {
			return cr.rule.CategoryID, true
		}
	}
	return 0, false
}

func (cr compiledRule) matches(tx *domain.Transaction) bool {
	raw := fieldValue(tx, cr.rule.Field)
	if cr.re != nil {
		return cr.re.MatchString(raw)
	}
	v := strings.ToLower(raw)
	switch cr.rule.Op {
	case domain.RuleOpContains:
		return strings.Contains(v, cr.value)
	case domain.RuleOpEquals:
		return v == cr.value
	case domain.RuleOpPrefix:
		return strings.HasPrefix(v, cr.value)
	}
	return false
}

func fieldValue(tx *domain.Transaction, f domain.RuleField) string {
	switch f {
	case domain.RuleFieldMerchant:
		return tx.Merchant
	case domain.RuleFieldDescription:
		return tx.Description
	case domain.RuleFieldMCC:
		return tx.MCC
	case domain.RuleFieldKind:
		return string(tx.Kind)
	case domain.RuleFieldCounterparty:
		return tx.Counterparty
	}
	return ""
}
