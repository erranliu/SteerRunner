package orca

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"
)

type TerminalWait struct {
	Handle, Condition string
	TimeoutMillis     int64
}

type TerminalWaitReceipt struct {
	Satisfied bool `json:"satisfied"`
	decoded   bool
}

func (r TerminalWaitReceipt) Decoded() bool { return r.decoded }

// WaitTerminal observes one explicit native condition. The caller supplies both
// the wire wait and invocation budget; satisfied is never a business outcome.
func (c Client) WaitTerminal(ctx context.Context, timeout time.Duration, wait TerminalWait) (Result, TerminalWaitReceipt, error) {
	if wait.Handle == "" || wait.Condition == "" || wait.TimeoutMillis < 0 {
		return Result{}, TerminalWaitReceipt{}, errors.New("Orca terminal wait requires an explicit terminal/condition and nonnegative wire timeout")
	}
	result, callErr := c.MetaCallWithTimeout(ctx, timeout, "terminal", "wait", "--terminal", wait.Handle, "--for", wait.Condition, "--timeout-ms", strconv.FormatInt(wait.TimeoutMillis, 10), "--json")
	var value struct{ Wait TerminalWaitReceipt }
	err := json.Unmarshal(result.Payload, &value)
	value.Wait.decoded = err == nil
	return result, value.Wait, errors.Join(callErr, err)
}
