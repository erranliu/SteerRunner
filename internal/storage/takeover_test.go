package storage

import (
	"github.com/erranliu/SteerRunner/internal/contracts"
	"testing"
)

func TestTakeoverRequestFactsUseOriginalAcceptedCommands(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	_, err = s.db.Exec(`INSERT INTO commands(request_id,command_id,command,instance_id,digest,response,action_status,run_id) VALUES('original-submit','command','submit','','original-digest','{}','completed','run')`)
	if err != nil {
		t.Fatal(err)
	}
	items := s.BindTakeoverRequests([]contracts.TakeoverItem{{ID: "run", Kind: "run", Status: "compatible"}, {ID: "missing", Kind: "run", Status: "compatible"}})
	if items[0].RequestID != "original-submit" || items[0].RequestDigest != "original-digest" || items[0].Status != "compatible" || items[1].Status != "unsupported" {
		t.Fatal(items)
	}
}
