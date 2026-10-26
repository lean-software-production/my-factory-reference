package main

import "testing"

// A machine's result is the last line of its answer that is a JSON object.
// The last non-blank line may also end with one after some prose, as models
// sometimes put it there.
func TestTheResultIsReadFromTheAnswer(t *testing.T) {
	for _, c := range []struct{ name, answer, want string }{
		{"a line on its own", "I did it.\n\n{\"task\": \"gravity\"}\n", `{"task": "gravity"}`},
		{"after prose on the last line", "Done.\nPieces now fall with the expected behavior. {\"task\":\"gravity\"}", `{"task":"gravity"}`},
		{"after a brace in prose", `see {x} then {"complete":true}`, `{"complete":true}`},
		{"on an earlier line, with prose last", "{\"complete\": false}\nThat is all.", `{"complete": false}`},
		{"prose only", "I did the task, but I have no result for you.", ""},
		{"a brace in prose only", "I used {braces} and {\"broken\": } JSON.", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			line, res := lastResult(c.answer)
			if line != c.want {
				t.Errorf("the result read is %q, not %q", line, c.want)
			}
			if (res == nil) != (c.want == "") {
				t.Errorf("the result is %v", res)
			}
		})
	}
}
