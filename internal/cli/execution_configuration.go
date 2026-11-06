package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/resources"
	"io"
	"os"
	"time"
)

func runExecutionConfiguration(input io.Reader, finish func(contracts.Response, int) int) int {
	return runExecutionConfigurationBudget(input, finish, 30*time.Second)
}

func runExecutionConfigurationBudget(input io.Reader, finish func(contracts.Response, int) int, budget time.Duration) int {
	type outcome struct {
		response contracts.Response
		exit     int
	}
	completed := make(chan outcome, 1)
	go func() { response, exit := executionConfigurationResponse(input); completed <- outcome{response, exit} }()
	timer := time.NewTimer(budget)
	defer timer.Stop()
	select {
	case result := <-completed:
		return finish(result.response, result.exit)
	case <-timer.C:
		if closer, ok := input.(io.Closer); ok {
			_ = closer.Close()
		}
		fault := contracts.Fail(4, "execution_configuration_timeout", "Execution configuration exceeded its deadline.")
		return finish(contracts.Rejected("", fault), fault.Exit)
	}
}

func executionConfigurationResponse(input io.Reader) (contracts.Response, int) {
	finish := func(response contracts.Response, exit int) (contracts.Response, int) { return response, exit }
	fail := func(fault *contracts.Fault) (contracts.Response, int) {
		return finish(contracts.Rejected("", fault), fault.Exit)
	}
	raw, err := io.ReadAll(io.LimitReader(input, contracts.ExecutionConfigurationLimit+1))
	if err != nil {
		return fail(contracts.Fail(2, "invalid_input", "Cannot read configuration input."))
	}
	executable, err := os.Executable()
	if err != nil {
		return fail(contracts.Fail(1, "internal_error", "Cannot identify configuration executable."))
	}
	file, err := os.Open(executable)
	if err != nil {
		return fail(contracts.Fail(1, "internal_error", "Cannot identify configuration executable."))
	}
	digest := sha256.New()
	_, err = io.Copy(digest, file)
	file.Close()
	if err != nil {
		return fail(contracts.Fail(1, "internal_error", "Cannot verify configuration executable."))
	}
	source := contracts.ExecutionConfigurationSource{Version: resources.Version, ExecutableSHA256: hex.EncodeToString(digest.Sum(nil)), ResourceDigest: resources.Digest()}
	result, fault := resources.ResolveExecutionConfiguration(raw, source)
	if fault != nil {
		return fail(fault)
	}
	response := contracts.Success("", result)
	encoded, err := json.Marshal(response)
	if err != nil || len(encoded)+1 > contracts.ExecutionConfigurationLimit {
		return fail(contracts.Fail(2, "invalid_input", "Configuration output exceeds its limit."))
	}
	return finish(response, 0)
}
