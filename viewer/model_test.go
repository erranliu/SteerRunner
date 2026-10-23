package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func fixture(at time.Time) Snapshot {
	return Snapshot{Version: 1, Source: strings.Repeat("a", 64), Instance: "original", Sequence: 1, Generated: at, Runs: []Run{{ID: "run", Title: "test", Workflow: Workflow{"new-workflow", 3}, Step: "new-step", Status: "new-status"}}}
}
func TestRemainingLifetimeRepeatedReadsAndSleep(t *testing.T) {
	now := time.Now().UTC()
	s := fixture(now.Add(-14 * time.Second))
	v := View{}
	if err := v.Accept(s, now, 1000); err != nil {
		t.Fatal(err)
	}
	if v.Deadline != 2000 || v.Stale(now, 1999) || !v.Stale(now, 2000) {
		t.Fatal("14-second-old snapshot received a fresh 15 seconds", v)
	}
	v.Error = "temporarily unreadable"
	_ = v.Accept(s, now.Add(time.Second), 2000)
	if v.Error != "" || v.Deadline != 2000 {
		t.Fatal("error recovery extended freshness")
	}
	if !v.Stale(now.Add(-time.Hour), 2001) {
		t.Fatal("clock rollback revived old sequence")
	}
	s.Sequence++
	s.Generated = now
	_ = v.Accept(s, now, 3000)
	if !v.Stale(now.Add(time.Hour), 3001) {
		t.Fatal("forward wall-clock adjustment did not expire cached state")
	}
	if !v.Stale(now, 3002) {
		t.Fatal("wall-clock correction revived an already expired snapshot")
	}
	s.Sequence++
	_ = v.Accept(s, now, 4000)
	if v.Stale(now, 4000) || !v.Stale(now, 3604000) {
		t.Fatal("new sequence did not recover or elapsed sleep revived cached state")
	}
	s.Instance = "next"
	s.Sequence = 1
	s.Generated = now.Add(-time.Hour)
	_ = v.Accept(s, now, 3605000)
	if !v.Stale(now, 3605000) {
		t.Fatal("new instance made old bytes fresh")
	}
}
func TestSequenceUnknownFieldsAndTimeAnomalies(t *testing.T) {
	now := time.Now().UTC()
	s := fixture(now)
	b, _ := json.Marshal(s)
	name := s.Source + ".json"
	withExtension := strings.Replace(string(b), `"schema_version":1`, `"schema_version":1,"future":{"x":true}`, 1)
	p, err := Parse([]byte(withExtension), name)
	if err != nil {
		t.Fatal(err)
	}
	v := View{}
	_ = v.Accept(s, now, 1)
	if v.Accept(p, now, 2) != nil || v.Deadline != 15001 {
		t.Fatal("ignored extension changed semantic sequence")
	}
	p, _ = Parse(b, name)
	p.Runs[0].Status = "changed"
	if v.Accept(p, now, 3) == nil {
		t.Fatal("same sequence mutation accepted")
	}
	p.Sequence = 2
	p.Generated = now.Add(6 * time.Second)
	if v.Accept(p, now, 4) == nil || v.Snapshot.Sequence != 1 {
		t.Fatal("future snapshot replaced last good")
	}
}
func TestStrictSnapshotAndSafeLinks(t *testing.T) {
	s := fixture(time.Now().UTC())
	b, _ := json.Marshal(s)
	name := s.Source + ".json"
	for _, invalid := range []string{strings.Replace(string(b), `"sequence":1`, `"sequence":1,"sequence":2`, 1), strings.Replace(string(b), `"pr":null`, `"pr":{"number":1,"url":"file:///C:/execute"}`, 1), strings.Replace(string(b), `"schema_version":1`, `"schema_version":2`, 1), strings.Replace(string(b), `"step":"new-step",`, "", 1), strings.Replace(string(b), `"runs":[`, `"runs":null,"other":[`, 1)} {
		if _, err := Parse([]byte(invalid), name); err == nil {
			t.Fatal("invalid snapshot accepted", invalid)
		}
	}
	if _, err := Parse(b, "other.json"); err == nil {
		t.Fatal("foreign source file accepted")
	}
	if !SafeURL("https://github.com/o/r/pull/1") || SafeURL("javascript:execute()") || SafeURL("https://user:password@example.com") {
		t.Fatal("unsafe link policy")
	}
}
