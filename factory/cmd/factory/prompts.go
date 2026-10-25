package main

import (
	"fmt"
	"strings"
)

// The prompts say what the factory knows and asks of a machine: where the
// seed and the plan are, what the doer just did, what the validator found,
// and the result it wants back. What a machine is for, and how it does its
// job, are in its AGENTS.md.
//
// The result a machine is asked for names the fields its edges on the line
// route on, and nothing else the line doesn't need.

// field is a part of the result a machine is asked for.
type field struct {
	name    string
	meaning string // what to say in it
	example string // a value of it, as JSON
}

// booleanFields describe the fields a machine's edges route on: each is true
// or false, and the first one means what the machine's job is to decide.
func booleanFields(names []string, meaning string) []field {
	var fields []field
	for i, name := range names {
		f := field{name, "set to true or false", "false"}
		if i == 0 && meaning != "" {
			f.meaning = meaning
		}
		fields = append(fields, f)
	}
	return fields
}

// resultRequest asks for the result: one line of JSON with the fields.
func resultRequest(fields []field) string {
	var asks, example []string
	for _, f := range fields {
		asks = append(asks, fmt.Sprintf("the field %q %s", f.name, f.meaning))
		example = append(example, fmt.Sprintf("%q: %s", f.name, f.example))
	}
	request := "Finish your answer with your result: one line of JSON, on its own"
	if len(asks) > 0 {
		request += ", with " + strings.Join(asks, ", and ")
	}
	return fmt.Sprintf("%s. For example:\n{%s}\n", request, strings.Join(example, ", "))
}

const (
	planComplete = "set to true if every task in the plan is done and false if any is left"
	workIsGood   = "set to true if the work is good enough and false if not"
)

func plannerFields(routed []string) []field {
	return booleanFields(routed, planComplete)
}

func plannerPrompt(seed, plan string, routed []string) string {
	return fmt.Sprintf(`The seed, which says what to build, is %s
There is no plan yet. It goes in %s

%s`, seed, plan, resultRequest(plannerFields(routed)))
}

func trackingPrompt(seed, plan string, routed []string) string {
	return fmt.Sprintf(`The seed, which says what to build, is %s
The plan is %s

The doer's work on a task is committed in this folder's Git history.

%s`, seed, plan, resultRequest(plannerFields(routed)))
}

func doerPrompt(seed, plan, findings string, routed []string) string {
	fields := append([]field{{"task", "naming the task you did", `"set up the project"`}},
		booleanFields(routed, "")...)
	return fmt.Sprintf(`The seed, which says what to build, is %s
The plan is %s

%s%s`, seed, plan, resultRequest(fields), validated(findings))
}

// validated is the part of the doer's prompt that passes on what the
// validator found wrong the last time, if it found anything.
func validated(findings string) string {
	if findings == "" {
		return ""
	}
	return fmt.Sprintf(`
The validator was not satisfied with your last attempt. Its findings, as JSON:
%s
`, findings)
}

func validatorPrompt(seed, plan, changes string, routed []string) string {
	fields := append(booleanFields(routed, workIsGood),
		field{"findings", "listing what is wrong", `["the new code has no test"]`})
	return fmt.Sprintf(`The seed, which says what to build, is %s
The plan is %s

The doer has just done a task in this folder. Its work is the changes here that aren't committed yet. Git lists them:

%s

%s`, seed, plan, changes, resultRequest(fields))
}

// genericPrompt is for a machine the factory knows nothing more of than its
// name: where the seed and the plan are, and the result its edges route on.
func genericPrompt(seed, plan string, routed []string) string {
	return fmt.Sprintf(`The seed, which says what to build, is %s
The plan is %s

%s`, seed, plan, resultRequest(booleanFields(routed, "")))
}
