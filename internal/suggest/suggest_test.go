package suggest

import "testing"

func TestMessagesWithHistory(t *testing.T) {
	history := []Turn{
		{Question: "export this wordpress db", Answer: Suggestion{Command: "mysqldump -u <user> -p <database> > dump.sql", Explanation: "Dumps the database."}},
		{Question: "with the wp cli", Answer: Suggestion{Command: "wp db export", Explanation: "Exports the database."}},
	}
	msgs := messages(history, "gzip it")

	if msgs[0].Role != "system" {
		t.Fatalf("first message should be the system prompt, got %q", msgs[0].Role)
	}
	tail := msgs[1+len(examples):]
	want := []struct{ role, content string }{
		{"user", "export this wordpress db"},
		{"assistant", `{"command":"mysqldump -u <user> -p <database> > dump.sql","explanation":"Dumps the database.","dangerous":false}`},
		{"user", "with the wp cli"},
		{"assistant", `{"command":"wp db export","explanation":"Exports the database.","dangerous":false}`},
		{"user", "gzip it"},
	}
	if len(tail) != len(want) {
		t.Fatalf("got %d messages after the examples, want %d", len(tail), len(want))
	}
	for i, w := range want {
		if tail[i].Role != w.role || tail[i].Content != w.content {
			t.Errorf("message %d = %s %q, want %s %q", i, tail[i].Role, tail[i].Content, w.role, w.content)
		}
	}
}

func TestMessagesWithoutHistory(t *testing.T) {
	msgs := messages(nil, "list files")
	if len(msgs) != 1+len(examples)+1 {
		t.Fatalf("got %d messages", len(msgs))
	}
	if last := msgs[len(msgs)-1]; last.Role != "user" || last.Content != "list files" {
		t.Fatalf("last message = %+v", last)
	}
}
