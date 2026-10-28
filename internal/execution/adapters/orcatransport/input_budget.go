package orcatransport

import (
	"errors"
	m "github.com/erranliu/SteerRunner/internal/execution/model"
	"strings"
)

func nativeWireText(turn *Turn) string {
	text := wirePrompt(turn)
	if turn.WireFormat == "" && turn.Native.agentName() == "codex" {
		text = nativePromptText(text)
	}
	return text
}
func freezeWireInput(turn *Turn, version uint64) error {
	var parts []m.InputPart
	var err error
	if turn.Budget != nil {
		parts, err = m.InputParts(turn.Prompt, turn.Budget)
	} else if version > 0 {
		return errors.New("original input provenance unavailable; candidate not sent")
	} else {
		parts = []m.InputPart{{Source: "caller.direct", Owner: m.CallerInput, Text: turn.Prompt}}
		_, turn.Budget, err = m.DescribeInput(parts...)
	}
	if err != nil {
		return err
	}
	if turn.WireFormat != "" || turn.Native.agentName() == "codex" {
		parts = m.NativeInputParts(parts)
	}
	body, _, err := m.DescribeInput(parts...)
	if err != nil {
		return err
	}
	text := nativeWireText(turn)
	if !strings.HasSuffix(text, body) {
		return errors.New("native wire does not preserve frozen body")
	}
	parts = append([]m.InputPart{{Source: "native.header", Owner: m.RunnerInput, Text: text[:len(text)-len(body)]}}, parts...)
	frozen, budget, err := m.FreezeInput(parts...)
	if err != nil {
		return err
	}
	if frozen != text {
		return errors.New("native wire changed during accounting")
	}
	turn.WireBudget = budget
	return nil
}
