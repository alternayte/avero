package codegen

import (
	"go/ast"
	"strconv"
	"strings"
)

// The rules that the SDD names. A custom rule is a Check method on the input
// type, which the generated Validate calls last. See the SDD, S5.
const (
	ruleRequired = "required"
	ruleMin      = "min"
	ruleMax      = "max"
	ruleEmail    = "email"
	ruleUUID     = "uuid"
	ruleOneOf    = "oneof"
)

// knownRules lists the rules in the order that a person reads them in a fault.
var knownRules = []string{ruleRequired, ruleMin, ruleMax, ruleEmail, ruleUUID, ruleOneOf}

// readRules parses one validate tag and records a fault for each rule that the
// generator cannot apply to the field.
func readRules(tag string, name *ast.Ident, k kind, c *collector) []rule {
	if strings.TrimSpace(tag) == "" {
		return nil
	}
	var out []rule
	for _, part := range strings.Split(tag, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		head, arg, hasArg := strings.Cut(part, "=")
		head = strings.TrimSpace(head)
		arg = strings.TrimSpace(arg)

		r := rule{Name: head, Arg: arg}
		switch head {
		case ruleRequired:
			if k == kindBool {
				c.at(name.Pos(),
					"the rule required does not apply to "+name.Name+", which is a bool",
					"Remove required from "+name.Name+", because a false bool is a value that a person sent")
				continue
			}
		case ruleMin, ruleMax:
			if !hasArg || arg == "" {
				c.at(name.Pos(),
					"the rule "+head+" on "+name.Name+" carries no number",
					"Write the rule as `"+head+"=3`")
				continue
			}
			if _, err := strconv.ParseFloat(arg, 64); err != nil {
				c.at(name.Pos(),
					"the rule "+head+" on "+name.Name+" carries "+strconv.Quote(arg)+", which is not a number",
					"Write the rule as `"+head+"=3`")
				continue
			}
			if !k.text() && !k.number() {
				c.at(name.Pos(),
					"the rule "+head+" does not apply to "+name.Name+", which has type "+typeOf(k),
					"Remove "+head+" from "+name.Name)
				continue
			}
		case ruleEmail, ruleUUID:
			if !k.text() {
				c.at(name.Pos(),
					"the rule "+head+" does not apply to "+name.Name+", which has type "+typeOf(k),
					"Remove "+head+" from "+name.Name+", or change the field to a string")
				continue
			}
		case ruleOneOf:
			r.Values = strings.Fields(arg)
			if len(r.Values) == 0 {
				c.at(name.Pos(),
					"the rule oneof on "+name.Name+" carries no value",
					"Write the rule as `oneof=admin member`")
				continue
			}
			if !k.text() {
				c.at(name.Pos(),
					"the rule oneof does not apply to "+name.Name+", which has type "+typeOf(k),
					"Remove oneof from "+name.Name+", or change the field to a string")
				continue
			}
		default:
			c.at(name.Pos(),
				"the rule "+strconv.Quote(head)+" on "+name.Name+" is not a rule that Avero knows",
				"Use one of "+strings.Join(knownRules, ", ")+", or write a Check method on the input type")
			continue
		}
		out = append(out, r)
	}
	return out
}

// typeOf names a kind in a fault message.
func typeOf(k kind) string {
	switch k {
	case kindString:
		return "string"
	case kindBool:
		return "bool"
	case kindInt:
		return "a signed integer"
	case kindUint:
		return "an unsigned integer"
	case kindFloat:
		return "a float"
	case kindDuration:
		return "time.Duration"
	case kindTime:
		return "time.Time"
	case kindStringSlice:
		return "[]string"
	default:
		return "an unknown type"
	}
}
