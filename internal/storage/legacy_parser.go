package storage

import (
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/contracts"
)

// Recognize the retired shape before typed decoding, without a codec or resource query.
func rejectLegacyParser(raw []byte) error {
	var r struct {
		Intents []struct {
			Request struct {
				Operation struct{ Action string }
				Input     map[string]json.RawMessage
			}
		}
		Snapshot struct{ Operations []struct{ Action string } }
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return err
	}
	fail := func() error {
		return contracts.Fail(3, "unsupported_parser_record", "State contains retired native parser execution records; finish or stop them with the original version and use a fresh state/v6 directory. Original data was not changed.")
	}
	for _, i := range r.Intents {
		if i.Request.Operation.Action == "parse_result" {
			return fail()
		}
		for key, v := range i.Request.Input {
			if (key == "Parse" || key == "parse") && string(v) != "null" {
				return fail()
			}
		}
	}
	for _, op := range r.Snapshot.Operations {
		if op.Action == "parse_result" {
			return fail()
		}
	}
	return nil
}
