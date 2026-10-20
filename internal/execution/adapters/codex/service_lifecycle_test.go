package codexworker

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	executionbackend "github.com/erranliu/SteerRunner/internal/execution/backend"
	resourceprocess "github.com/erranliu/SteerRunner/internal/execution/process"
)

func TestServiceStopWorkerHelper(t *testing.T) {
	if os.Getenv("STEERRUNNER_TEST_STOP_HELPER") != "1" {
		t.Skip("owned process helper")
	}
	for {
		time.Sleep(time.Minute)
	}
}

func TestServiceStopWorkerFacts(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestServiceStopWorkerHelper$")
	cmd.Env = append(os.Environ(), "STEERRUNNER_TEST_STOP_HELPER=1")
	owned, err := resourceprocess.StartManaged(cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer owned.Close()
	defer cmd.Wait()
	identity := owned.Identity()
	if owned.WritersAbsent() {
		t.Fatal("live owned job was reported empty")
	}
	snapshot := executionbackend.LifecycleSnapshot{Ref: executionbackend.ExecutionRef{External: identity},
		Session:             executionbackend.SessionContext{ProjectID: "project", WorkflowRunID: "workflow", ExecutionID: "execution", AttemptID: "attempt"},
		ConfigurationDigest: "config", ResourceGeneration: "generation"}
	l := ExecutionLifecycle{setup: executionbackend.LifecycleSetup{OwnedProcess: owned}, Backend: Backend{Process: ProcessPort{Inspect: resourceprocess.InspectProcess}}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	facts, err := l.ShutdownLifecycle(ctx, snapshot, owned)
	if err != nil || !facts.WorkerAbsent || !facts.WritersAbsent || !facts.Stopped || facts.TurnCompleted {
		t.Fatalf("owned worker release facts: %+v, %v", facts, err)
	}
	if l.PolicyCapabilities().ServiceStopBehavior != executionbackend.StopExecutionOnServiceStop {
		t.Fatal("local worker did not declare explicit stop behavior")
	}
}

func TestIndependentOwnerServiceDetachPreservesOriginalTree(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestServiceStopWorkerHelper$")
	cmd.Env = append(os.Environ(), "STEERRUNNER_TEST_STOP_HELPER=1")
	owned, err := resourceprocess.StartManaged(cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer cmd.Wait()
	defer owned.Close()
	defer owned.Kill()
	id := owned.Identity()
	s := executionbackend.LifecycleSnapshot{Ref: executionbackend.ExecutionRef{External: id}, Session: executionbackend.SessionContext{ProjectID: "p", WorkflowRunID: "w", ExecutionID: "e", AttemptID: "a"}, ConfigurationDigest: "c", ResourceGeneration: "g"}
	l := ExecutionLifecycle{setup: executionbackend.LifecycleSetup{LaunchExecutionOwner: func(context.Context, string) error { return nil }}, Backend: Backend{Process: ProcessPort{Inspect: resourceprocess.InspectProcess}}}
	facts, err := l.ShutdownLifecycle(context.Background(), s, owned)
	if err != nil || facts.Stopped || owned.WritersAbsent() || resourceprocess.InspectProcess(&id) != "matching process still exists" {
		t.Fatalf("service detach ended original native tree: %+v %v", facts, err)
	}
	if l.PolicyCapabilities().ServiceStopBehavior != executionbackend.ContinueExecutionOnServiceStop {
		t.Fatal("independent host did not declare continue execution")
	}
}
