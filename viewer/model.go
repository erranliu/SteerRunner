package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"reflect"
	"regexp"
	"time"
)

type Workflow struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
}
type PR struct {
	URL    string `json:"url"`
	Number int    `json:"number"`
}
type Run struct {
	ID       string   `json:"run_id"`
	Title    string   `json:"title"`
	Workflow Workflow `json:"workflow"`
	Step     string   `json:"step"`
	Status   string   `json:"status"`
	PR       *PR      `json:"pr"`
}
type Snapshot struct {
	Version   int       `json:"schema_version"`
	Source    string    `json:"source_id"`
	Instance  string    `json:"instance_id"`
	Sequence  uint64    `json:"sequence"`
	Generated time.Time `json:"generated_at"`
	Runs      []Run     `json:"runs"`
}

var sourcePattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

// Check duplicate keys at every object depth, including ignored extensions.
func uniqueJSON(d *json.Decoder) error {
	t, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := t.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			k, err := d.Token()
			if err != nil {
				return err
			}
			s, ok := k.(string)
			if !ok || seen[s] {
				return errors.New("duplicate JSON key")
			}
			seen[s] = true
			if err = uniqueJSON(d); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err = uniqueJSON(d); err != nil {
				return err
			}
		}
	default:
		return errors.New("unexpected JSON delimiter")
	}
	_, err = d.Token()
	return err
}
func Parse(body []byte, name string) (Snapshot, error) {
	var s Snapshot
	d := json.NewDecoder(bytes.NewReader(body))
	d.UseNumber()
	if err := uniqueJSON(d); err != nil {
		return s, err
	}
	if _, err := d.Token(); err != io.EOF {
		return s, errors.New("trailing JSON")
	}
	if err := json.Unmarshal(body, &s); err != nil {
		return s, err
	}
	if s.Version != 1 || !sourcePattern.MatchString(s.Source) || filepath.Base(name) != s.Source+".json" || s.Instance == "" || s.Sequence == 0 || s.Generated.IsZero() || s.Generated.Location() != time.UTC || s.Runs == nil {
		return s, errors.New("unsupported or invalid snapshot metadata")
	}
	// Presence matters even where an empty string is a valid phase/title value.
	var raw struct {
		Runs []map[string]json.RawMessage `json:"runs"`
	}
	_ = json.Unmarshal(body, &raw)
	seen := map[string]bool{}
	for i, r := range s.Runs {
		for _, key := range []string{"run_id", "title", "workflow", "step", "status", "pr"} {
			v, ok := raw.Runs[i][key]
			if !ok || (key != "pr" && bytes.Equal(v, []byte("null"))) {
				return s, fmt.Errorf("missing run field %s", key)
			}
		}
		if r.ID == "" || r.Status == "" || r.Workflow.ID == "" || r.Workflow.Version < 1 || seen[r.ID] {
			return s, errors.New("invalid or duplicate run")
		}
		seen[r.ID] = true
		if r.PR != nil && (r.PR.Number < 1 || !SafeURL(r.PR.URL)) {
			return s, errors.New("invalid PR link")
		}
	}
	return s, nil
}
func SafeURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" && u.User == nil
}

type View struct {
	Snapshot *Snapshot
	Deadline uint64
	Error    string
	expired  bool
}

func (v *View) Accept(s Snapshot, wall time.Time, tick uint64) error {
	if s.Generated.After(wall.Add(5 * time.Second)) {
		v.Error = "time anomaly"
		return errors.New(v.Error)
	}
	if v.Snapshot != nil && s.Instance == v.Snapshot.Instance {
		if s.Sequence < v.Snapshot.Sequence || (s.Sequence == v.Snapshot.Sequence && !reflect.DeepEqual(s, *v.Snapshot)) {
			v.Error = "snapshot sequence anomaly"
			return errors.New(v.Error)
		}
		if s.Sequence == v.Snapshot.Sequence {
			v.Error = ""
			return nil
		}
	}
	age := wall.Sub(s.Generated)
	if age < 0 {
		age = 0
	}
	remaining := 15*time.Second - age
	if remaining < 0 {
		remaining = 0
	}
	v.Snapshot = &s
	v.Deadline = tick + uint64(remaining/time.Millisecond)
	v.expired = remaining == 0
	v.Error = ""
	return nil
}
func (v *View) Stale(wall time.Time, tick uint64) bool {
	v.expired = v.expired || v.Snapshot == nil || tick >= v.Deadline || wall.Sub(v.Snapshot.Generated) >= 15*time.Second
	return v.expired
}
