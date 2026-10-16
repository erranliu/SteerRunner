package orcatransport

import (
	"context"
	"strings"
	"testing"
)

func TestRemoteAuthenticationCapabilityMissingBlocksBeforeBusiness(t *testing.T) {
	l, host, s := fixtureOn(t, "paired-remote")
	l.setup.RepositoryURL = "https://github.com/erranliu/SteerlineTest.git"
	_, err := l.PreparationFacts(context.Background(), s)
	if err == nil || !strings.Contains(err.Error(), "remote_sandbox_authentication_unsupported") {
		t.Fatalf("unsupported remote admitted: %v", err)
	}
	if len(host.calls) != 0 {
		t.Fatal("unsupported preparation performed native work")
	}
}
