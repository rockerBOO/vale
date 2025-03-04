package check

import (
	"github.com/errata-ai/regexp2"
	"github.com/errata-ai/vale/v2/internal/core"
	"github.com/errata-ai/vale/v2/internal/nlp"
	"github.com/mitchellh/mapstructure"
)

// Occurrence counts the number of times Token appears.
type Occurrence struct {
	Definition `mapstructure:",squash"`
	// `ignorecase` (`bool`): Makes all matches case-insensitive.
	Ignorecase bool
	// `max` (`int`): The maximum amount of times `token` may appear in a given
	// scope.
	Max int
	// `min` (`int`): The minimum amount of times `token` has to appear in a
	// given scope.
	Min int
	// `token` (`string`): The token of interest.
	Token string

	pattern *regexp2.Regexp
}

// NewOccurrence creates a new `occurrence`-based rule.
func NewOccurrence(cfg *core.Config, generic baseCheck) (Occurrence, error) {
	rule := Occurrence{}
	path := generic["path"].(string)

	err := mapstructure.WeakDecode(generic, &rule)
	if err != nil {
		return rule, readStructureError(err, path)
	}

	regex := ""
	if rule.Ignorecase {
		regex += ignoreCase
	}

	regex += `(?:` + rule.Token + `)`
	re, err := regexp2.CompileStd(regex)
	if err != nil {
		return rule, core.NewE201FromPosition(err.Error(), path, 1)
	}

	rule.pattern = re
	return rule, nil
}

// Run checks the number of occurrences of a user-defined regex against a
// certain threshold.
func (o Occurrence) Run(blk nlp.Block, f *core.File) []core.Alert {
	var a core.Alert

	alerts := []core.Alert{}
	txt := blk.Text
	locs := o.pattern.FindAllStringIndex(txt, -1)

	occurrences := len(locs)
	if (o.Max > 0 && occurrences > o.Max) || (o.Min > 0 && occurrences < o.Min) {
		if occurrences == 0 {
			// NOTE: We might not have a location to report -- i.e., by
			// definition, having zero instances of a token may break a rule.
			//
			// In a case like this, the check essentially becomes
			// document-scoped (like `readability`), so we mark the issue at
			// the first line.
			a = core.Alert{
				Check: o.Name, Severity: o.Level, Span: []int{1, 1},
				Link: o.Link}
		} else {
			// NOTE: We take only the first match (`locs[0]`) instead of the
			// whole scope (`txt`) to avoid having to fall back to string
			// matching.
			//
			// See (core/location.go#initialPosition).
			a = makeAlert(o.Definition, locs[0], txt)
		}

		a.Message = o.Message
		a.Description = o.Description
		alerts = append(alerts, a)
	}

	return alerts
}

// Fields provides access to the internal rule definition.
func (o Occurrence) Fields() Definition {
	return o.Definition
}

// Pattern is the internal regex pattern used by this rule.
func (o Occurrence) Pattern() string {
	return o.pattern.String()
}
