package storage

import (
	"bytes"
	"testing"
)

func TestLegacyParserRecordBoundary(t *testing.T) {
	for _, s := range []string{
		`{"Intents":[{"Request":{"Operation":{"action":"parse_result"}}}]}`,
		`{"Intents":[{"Request":{"Input":{"parse":{"payload":"old"}}}}]}`,
		`{"snapshot":{"Operations":[{"action":"parse_result","status":"confirmed"}]}}`,
	} {
		if rejectLegacyParser([]byte(s)) == nil {
			t.Fatal("legacy parser accepted", s)
		}
	}
	for _, s := range []string{
		`{"Profile":[{"retains_parser":true}],"result_acquisition_config":{"config":{"codex_parser":{"enabled":true}}}}`,
		`{"Workflow":{"Results":[{"method":"llm_parser"}]}}`,
		`{"Intents":[{"Request":{"Input":{"parse":null},"Operation":{"action":"start_turn"}}}]}`,
	} {
		if err := rejectLegacyParser([]byte(s)); err != nil {
			t.Fatal("non-parser metadata rejected", err)
		}
	}
}

func TestLegacyParserStoreRejectsWithoutRewriting(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	raw := []byte(`{"id":"old","Intents":[{"Request":{"Operation":{"action":"parse_result"}}}]}`)
	if _, err = s.db.Exec("INSERT INTO runs(id,revision,body) VALUES(?,?,?)", "old", 1, raw); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Runs(); err == nil {
		t.Fatal("legacy operation reached typed execution records")
	}
	var after []byte
	if err = s.db.QueryRow("SELECT body FROM runs WHERE id=?", "old").Scan(&after); err != nil || !bytes.Equal(raw, after) {
		t.Fatal("legacy record changed", err)
	}
}
