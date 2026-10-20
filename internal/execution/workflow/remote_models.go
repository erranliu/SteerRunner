package workflow

import "github.com/erranliu/SteerRunner/internal/execution/model"

// RemoteModelBindings preserves the existing managed TUI bridge whitelist. It
// is assembled from the original configuration, never persisted as new defaults.
func RemoteModelBindings(writer ModelSettings, audit *ModelSettings) model.Bindings {
	bindings := model.Bindings{}
	for _, step := range []string{StepImplement, StepReport, StepPublish, StepPublishReport} {
		value := writer
		bindings[step] = &value
	}
	for _, step := range []string{StepAuditCode, StepAuditReport} {
		bindings[step] = audit
	}
	return bindings.Clone()
}
