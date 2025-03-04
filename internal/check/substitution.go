package check

import (
	"fmt"
	"strings"

	"github.com/errata-ai/regexp2"
	"github.com/errata-ai/vale/v2/internal/core"
	"github.com/errata-ai/vale/v2/internal/nlp"
	"github.com/mitchellh/mapstructure"
)

// Substitution switches the values of Swap for its keys.
type Substitution struct {
	Definition `mapstructure:",squash"`
	// `ignorecase` (`bool`): Makes all matches case-insensitive.
	Ignorecase bool
	// `nonword` (`bool`): Removes the default word boundaries (`\b`).
	Nonword bool
	// `swap` (`map`): A sequence of `observed: expected` pairs.
	Swap map[string]string
	// `pos` (`string`): A regular expression matching tokens to parts of
	// speech.
	POS string
	// `exceptions` (`array`): An array of strings to be ignored.
	Exceptions []string

	exceptRe *regexp2.Regexp
	pattern  *regexp2.Regexp
	repl     []string
}

// NewSubstitution creates a new `substitution`-based rule.
func NewSubstitution(cfg *core.Config, generic baseCheck) (Substitution, error) {
	rule := Substitution{}
	path := generic["path"].(string)

	err := mapstructure.WeakDecode(generic, &rule)
	if err != nil {
		return rule, readStructureError(err, path)
	}
	tokens := ""

	re, err := updateExceptions(rule.Exceptions, cfg.AcceptedTokens)
	if err != nil {
		return rule, core.NewE201FromPosition(err.Error(), path, 1)
	}
	rule.exceptRe = re

	regex := makeRegexp(
		cfg.WordTemplate,
		rule.Ignorecase,
		func() bool { return !rule.Nonword },
		func() string { return "" }, true)

	replacements := []string{}
	for regexstr, replacement := range rule.Swap {
		opens := strings.Count(regexstr, "(")
		if opens != strings.Count(regexstr, "(?") &&
			opens != strings.Count(regexstr, `\(`) {
			// We rely on manually-added capture groups to associate a match
			// with its replacement -- e.g.,
			//
			//    `(foo)|(bar)`, [replacement1, replacement2]
			//
			// where the first capture group ("foo") corresponds to the first
			// element of the replacements slice ("replacement1"). This means
			// that we can only accept non-capture groups from the user (the
			// indexing would be mixed up otherwise).
			//
			// TODO: Should we change this? Perhaps by creating a map of regex
			// to replacements?
			return rule, core.NewE201FromTarget(
				"capture group not supported; use '(?:' instead of '('", regexstr, path)
		}
		tokens += `(` + regexstr + `)|`
		replacements = append(replacements, replacement)
	}
	regex = fmt.Sprintf(regex, strings.TrimRight(tokens, "|"))

	re, err = regexp2.CompileStd(regex)
	if err != nil {
		return rule, core.NewE201FromPosition(err.Error(), path, 1)
	}

	rule.pattern = re
	rule.repl = replacements
	return rule, nil
}

// Run executes the the `substitution`-based rule.
//
// The rule looks for one pattern and then suggests a replacement.
func (s Substitution) Run(blk nlp.Block, f *core.File) []core.Alert {
	alerts := []core.Alert{}

	txt := blk.Text
	// Leave early if we can to avoid calling `FindAllStringSubmatchIndex`
	// unnecessarily.
	if !s.pattern.MatchStringStd(txt) {
		return alerts
	}

	for _, submat := range s.pattern.FindAllStringSubmatchIndex(txt, -1) {
		for idx, mat := range submat {
			if mat != -1 && idx > 0 && idx%2 == 0 {
				loc := []int{mat, submat[idx+1]}
				// Based on the current capture group (`idx`), we can determine
				// the associated replacement string by using the `repl` slice:
				expected := s.repl[(idx/2)-1]
				observed := strings.TrimSpace(re2Loc(txt, loc))

				same := matchToken(expected, observed, s.Ignorecase)
				if !same && !isMatch(s.exceptRe, observed) {
					action := s.Fields().Action
					if action.Name == "replace" && len(action.Params) == 0 {
						action.Params = strings.Split(expected, "|")
						expected = core.ToSentence(action.Params, "or")

						// NOTE: For backwards-compatibility, we need to ensure
						// that we don't double quote.
						s.Message = convertMessage(s.Message)
					}
					a := makeAlert(s.Definition, loc, txt)
					a.Message, a.Description = formatMessages(s.Message,
						s.Description, expected, observed)

					alerts = append(alerts, a)
				}
			}
		}
	}

	return alerts
}

// Fields provides access to the internal rule definition.
func (s Substitution) Fields() Definition {
	return s.Definition
}

// Pattern is the internal regex pattern used by this rule.
func (s Substitution) Pattern() string {
	return s.pattern.String()
}

func convertMessage(s string) string {
	for _, spec := range []string{"'%s'", "\"%s\""} {
		if strings.Count(s, spec) == 2 {
			s = strings.Replace(s, spec, "%s", 1)
		}
	}
	return s
}
