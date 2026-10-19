package workflow

import ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"

type Report = ra.Report
type Outputs = ra.Outputs
type Blocker = ra.Blocker

const ResultContract = ra.ResultContract

func contractFor(step string) string {
	if AuditStep(step) {
		return "audit"
	}
	if PublicationStep(step) {
		return "publication"
	}
	return "implementation"
}
func DecodeReport(body, step string) (Report, error) { return ra.DecodeReport(body, contractFor(step)) }
func ResultInstruction(step string) string           { return ra.ResultInstruction(contractFor(step)) }
