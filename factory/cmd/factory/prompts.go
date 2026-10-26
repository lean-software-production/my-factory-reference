package main

import (
	"fmt"
	"strings"
)

// Every machine is told the same things, whatever it is for: where the seed
// and the plan are, what the machine before it on the line answered, and
// what is uncommitted in the folder it works in. It is asked for the result
// its edges route on. What a machine is for, and how it does its job, are
// in its AGENTS.md.

// briefing is what the factory says to a machine.
type briefing struct {
	seed, plan string
	runDir     string // the run's folder, where the plan is
	planExists bool
	before     string // the name of the machine before it on the line, if any
	answer     string // that machine's result, as it said it
	changes    string // what Git lists as uncommitted in the folder, if anything
	fields     []field
}

// field is a part of the result a machine is asked for.
type field struct {
	name    string
	meaning string // what to say in it
	example string // a value of it, as JSON
}

// resultFields are the fields a machine is asked for. Each field its edges
// route on is true or false. A machine the planner sends work to also names
// the task it did, which names the commit of its work, and one that can send
// work back lists what is wrong with it.
func resultFields(routed []string, worker, canSendWorkBack bool) []field {
	var fields []field
	if worker {
		fields = append(fields, field{"task", "naming the task you did", `"set up the project"`})
	}
	for _, name := range routed {
		fields = append(fields, field{name, "set to true or false", "false"})
	}
	if canSendWorkBack {
		fields = append(fields, field{"findings", "listing what is wrong", `["the new code has no test"]`})
	}
	return fields
}

func (b briefing) String() string {
	var prompt strings.Builder
	fmt.Fprintf(&prompt, "The seed, which says what to build, is %s\n", b.seed)
	if b.planExists {
		fmt.Fprintf(&prompt, "The plan is %s\n", b.plan)
	} else {
		fmt.Fprintf(&prompt, "There is no plan yet. It goes in %s\n", b.plan)
	}
	fmt.Fprintf(&prompt, "The run's folder, where you keep anything you make for the run, is %s\n", b.runDir)
	prompt.WriteString("\nWhat has been committed so far is in this folder's Git history.\n")
	if b.changes != "" {
		fmt.Fprintf(&prompt, "\nGit lists these changes in this folder that aren't committed yet:\n\n%s\n", b.changes)
	}
	if b.answer != "" {
		fmt.Fprintf(&prompt, "\nThe machine before you on the line, the %s, answered with this result:\n%s\n", b.before, b.answer)
	}
	prompt.WriteString("\n" + resultRequest(b.fields))
	return prompt.String()
}

// meetingBriefing is what the factory says to the machine where the
// branches of a fan out meet: what each branch answered, and nothing else.
type meetingBriefing struct {
	branches []answer
	fields   []field
}

func (b meetingBriefing) String() string {
	var names, results []string
	for _, a := range b.branches {
		names = append(names, a.machine)
		results = append(results, a.machine+": "+a.result)
	}
	var prompt strings.Builder
	fmt.Fprintf(&prompt, "The machines before you on the line, %s, ran at the same time and answered with these results:\n\n%s\n",
		strings.Join(names, ", "), strings.Join(results, "\n"))
	prompt.WriteString("\n" + resultRequest(b.fields))
	return prompt.String()
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
