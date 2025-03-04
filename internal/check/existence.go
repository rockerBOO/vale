package check

import (
	"fmt"
	"strings"

	"github.com/errata-ai/regexp2"
	"github.com/errata-ai/vale/v2/internal/core"
	"github.com/errata-ai/vale/v2/internal/nlp"
	"github.com/mitchellh/mapstructure"
)

// Existence checks for the present of Tokens.
type Existence struct {
	Definition `mapstructure:",squash"`
	// `append` (`bool`): Adds `raw` to the end of `tokens`, assuming both are
	// defined.
	Append bool
	// `ignorecase` (`bool`): Makes all matches case-insensitive.
	IgnoreCase bool
	// `nonword` (`bool`): Removes the default word boundaries (`\b`).
	Nonword bool
	// `raw` (`array`): A list of tokens to be concatenated into a pattern.
	Raw []string
	// `tokens` (`array`): A list of tokens to be transformed into a
	// non-capturing group.
	Tokens []string
	// `exceptions` (`array`): An array of strings to be ignored.
	Exceptions []string

	exceptRe *regexp2.Regexp
	pattern  *regexp2.Regexp
}

// NewExistence creates a new `Rule` that extends `Existence`.
func NewExistence(cfg *core.Config, generic baseCheck) (Existence, error) {
	rule := Existence{}

	path := ""
	if p, ok := generic["path"].(string); !ok {
		path = p
	}

	err := mapstructure.WeakDecode(generic, &rule)
	if err != nil {
		return rule, readStructureError(err, path)
	}

	re, err := updateExceptions(rule.Exceptions, cfg.AcceptedTokens)
	if err != nil {
		return rule, core.NewE201FromPosition(err.Error(), path, 1)
	}
	rule.exceptRe = re

	regex := makeRegexp(
		cfg.WordTemplate,
		rule.IgnoreCase,
		func() bool { return !rule.Nonword && len(rule.Tokens) > 0 },
		func() string { return strings.Join(rule.Raw, "") },
		rule.Append)
	regex = fmt.Sprintf(regex, strings.Join(rule.Tokens, "|"))

	re, err = regexp2.CompileStd(regex)
	if err != nil {
		return rule, core.NewE201FromPosition(err.Error(), path, 1)
	}
	rule.pattern = re

	return rule, nil
}

// Run executes the the `existence`-based rule.
//
// This is simplest of the available extension points: it looks for any matches
// of its internal `pattern` (calculated from `NewExistence`) against the
// provided text.
func (e Existence) Run(blk nlp.Block, file *core.File) []core.Alert {
	alerts := []core.Alert{}

	for _, loc := range e.pattern.FindAllStringIndex(blk.Text, -1) {
		observed := strings.TrimSpace(re2Loc(blk.Text, loc))
		if !isMatch(e.exceptRe, observed) {
			alerts = append(alerts, makeAlert(e.Definition, loc, blk.Text))
		}
	}

	return alerts
}

// Fields provides access to the internal rule definition.
func (e Existence) Fields() Definition {
	return e.Definition
}

// Pattern is the internal regex pattern used by this rule.
func (e Existence) Pattern() string {
	return e.pattern.String()
}
