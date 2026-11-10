package textanalysis

import "testing"

func TestParserUsageFromOriginalCompletedTurn(t *testing.T) {
	for _, tc := range []struct {
		usage   string
		covered bool
	}{
		{`,"usage":{"input_tokens":12,"cached_input_tokens":3,"output_tokens":4}`, true},
		{"", false},
		{`,"usage":{"input_tokens":-1,"output_tokens":4}`, false},
		{`,"usage":{"input_tokens":12,"cached_input_tokens":99,"output_tokens":4}`, false},
		{`,"usage":{"input_tokens":"bad","output_tokens":4}`, false},
		{`,"usage":[]`, false},
	} {
		s := Stream{}
		for _, line := range []string{`{"type":"thread.started","thread_id":"parser-original"}`, `{"type":"turn.started"}`, `{"type":"item.completed","item":{"type":"agent_message","id":"final-id","text":"{}"}}`, `{"type":"turn.completed"` + tc.usage + `}`} {
			if err := s.Accept([]byte(line)); err != nil {
				t.Fatal(err)
			}
		}
		if (s.Tokens.Total != nil) != tc.covered || s.SessionID != "parser-original" || s.ResponseID != "final-id" {
			t.Fatal("usage identity or absence lost", s)
		}
	}
}

func TestParserUsageConflictInvalidatesCompletedCounters(t *testing.T) {
	for _, last := range []string{`{"type":"turn.completed","usage":{"input_tokens":999,"output_tokens":1}}`, `{"type":"error","message":"unknown stream outcome"}`, `{"type":"item.started","item":{"type":"command_execution"}}`, `broken JSON`} {
		s := Stream{}
		for _, line := range []string{`{"type":"thread.started","thread_id":"original"}`, `{"type":"turn.started"}`, `{"type":"turn.completed","usage":{"input_tokens":12,"output_tokens":4}}`} {
			if err := s.Accept([]byte(line)); err != nil {
				t.Fatal(err)
			}
		}
		if err := s.Accept([]byte(last)); err == nil || s.Tokens.Total != nil {
			t.Fatal("conflicted stream retained countable usage", last, s)
		}
	}
}
