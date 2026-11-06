package cli

import (
	"github.com/erranliu/SteerRunner/internal/contracts"
	"io"
	"testing"
	"time"
)

func TestExecutionConfigurationDeadlineIncludesStdin(t *testing.T) {
	reader, writer := io.Pipe()
	defer writer.Close()
	var response contracts.Response
	started := time.Now()
	exit := runExecutionConfigurationBudget(reader, func(r contracts.Response, exit int) int { response = r; return exit }, 50*time.Millisecond)
	if exit != 4 || response.OK || response.Data != nil || response.Error == nil || response.Error.Code != "execution_configuration_timeout" || time.Since(started) > time.Second {
		t.Fatal(exit, response, time.Since(started))
	}
	if _, err := writer.Write([]byte("{}")); err == nil {
		t.Fatal("timed out input still open")
	}
}
