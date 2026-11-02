package orca

import (
	"bytes"
	"encoding/json"
	"errors"
)

type Result struct {
	Payload       json.RawMessage
	RuntimeID     string
	ErrorCode     string
	ErrorMessage  string
	CallUncertain bool
}

type Envelope struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Meta struct {
		RuntimeID string `json:"runtimeId"`
	} `json:"_meta"`
}

func ParseEnvelope(raw []byte) (Envelope, error) {
	var value Envelope
	err := json.Unmarshal(bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf}), &value)
	if err != nil {
		return value, err
	}
	if !value.OK {
		if value.Error != nil {
			return value, errors.New(value.Error.Message)
		}
		return value, errors.New("Orca CLI rejected request")
	}
	return value, nil
}

// DecodeCLIResult preserves the caller's dispatch uncertainty; parsing a reply
// neither authorizes another invocation nor proves a business operation began.
func DecodeCLIResult(raw []byte, uncertain bool) (Result, error) {
	envelope, err := ParseEnvelope(raw)
	result := Result{Payload: envelope.Result, RuntimeID: envelope.Meta.RuntimeID, CallUncertain: uncertain}
	if envelope.Error != nil {
		result.ErrorCode, result.ErrorMessage = envelope.Error.Code, envelope.Error.Message
	}
	return result, err
}
