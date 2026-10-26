package host

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/erranliu/SteerRunner/internal/application"
	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	parser "github.com/erranliu/SteerRunner/internal/execution/resultacquisition/codexexec"
	"github.com/erranliu/SteerRunner/internal/resources"
	"github.com/erranliu/SteerRunner/internal/runview"
	"github.com/erranliu/SteerRunner/internal/storage"
	"github.com/erranliu/SteerRunner/internal/usage/api"
	"github.com/erranliu/SteerRunner/internal/usage/collect"
)

type server struct {
	engine      *application.Engine
	endpoint    endpoint
	store       *storage.Store
	mutex       sync.RWMutex // admission spans handlers; stop waits for admitted calls
	stopping    bool
	stop        chan string
	logger      *log.Logger
	usageCancel context.CancelFunc
}

func Serve(ctx context.Context, dir, startupID string) (outErr error) {
	created, detached, err := launchFacts(dir, startupID)
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(dir, orca.CreationOwnerConfigFile)); err == nil {
		completeBootstrap(dir, startupID)
		return orca.ServeCreationOwner(ctx, dir)
	}
	if executionOwnerDirectory(dir) {
		completeBootstrap(dir, startupID)
		return codex.ServeExecutionOwner(ctx, dir, func(ctx context.Context, root string, request b.LifecycleRequest) codex.OwnedParser {
			return parser.Start(ctx, root, request, func(b.LifecycleReceipt) error { return nil })
		})
	}
	for _, name := range []string{"service", "logs"} {
		if err := os.MkdirAll(filepath.Join(dir, name), 0700); err != nil {
			return err
		}
	}
	lock, busy, err := tryLock(filepath.Join(dir, "service", "writer.lock"), true)
	if err != nil {
		return err
	}
	if busy {
		return contracts.Fail(5, "directory_conflict", "This state directory already has a writer.")
	}
	defer lock.Close()
	// A new writer invalidates prior shutdown evidence even if startup fails.
	if err := os.Remove(shutdownPath(dir)); err != nil && !os.IsNotExist(err) {
		return err
	}
	output, err := os.OpenFile(filepath.Join(dir, "logs", "service.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer output.Close()
	logger := log.New(output, "", log.Ldate|log.Ltime|log.LUTC)
	defer func() {
		if outErr != nil {
			logger.Print("service failed: ", outErr)
		}
	}()
	if err := qualifyLegacyRecovery(dir); err != nil {
		logger.Print("recovery source rejected: ", err)
		return err
	}
	store, err := storage.Open(dir)
	if err != nil {
		logger.Print("startup rejected: ", err)
		return err
	}
	defer store.Close()
	engine, err := application.New(ctx, dir, store, logger, func(ctx context.Context, ownerDir string) error { return launchExecutionOwner(ctx, dir, ownerDir) })
	if err != nil {
		return err
	}
	var service contracts.ServiceInfo
	pending, err := loadPendingStart(dir)
	if err != nil {
		return err
	}
	if pending != nil {
		for _, stop := range pending.PendingStops {
			response := engine.Handle("stop", stop.RunID, "", "", "", 0, false, &stop.Request)
			if !response.OK {
				// An explicit rejection is not an unresolved acceptance. Preserve
				// accepted controls in commands/Run, and let the caller form a
				// fresh request after inspecting the rejection. Unknown commits
				// retain the exact startup intent for reconciliation.
				if response.Error.Code != "request_outcome_unknown" {
					if err = os.Remove(pendingStartPath(dir)); err != nil {
						return err
					}
				}
				return contracts.Fail(contracts.ExitCode(response), response.Error.Code, response.Error.Message)
			}
		}
		if err = os.Remove(pendingStartPath(dir)); err != nil {
			return err
		}
	}
	usagePolicy := api.DefaultPolicy()
	if pending != nil && pending.UsagePolicy != nil {
		usagePolicy = *pending.UsagePolicy
	}
	if err = store.InitUsage(); err != nil {
		return err
	}
	stopID := ""
	defer func() {
		shutdown, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		shutdownErr := engine.Shutdown(shutdown)
		if service.InstanceID == "" {
			return
		}
		settlementErr := engine.ResourceSettlementError()
		result := contracts.ServiceShutdown{InstanceID: service.InstanceID, RequestID: stopID, StateDir: dir, ExecutableSHA256: service.ExecutableSHA256, ResourcesSettled: shutdownErr == nil && settlementErr == nil}
		if !result.ResourcesSettled {
			result.Diagnostic = "Worker shutdown or native resource settlement remains unverified."
			if settlementErr != nil {
				result.Diagnostic += " " + settlementErr.Error()
			}
		}
		if shutdownErr == nil && stopID != "" {
			if err := store.CompleteStop(stopID); err != nil {
				logger.Print("shutdown receipt remains pending: ", err)
			}
		}
		if err := writeShutdown(dir, result); err != nil {
			logger.Print("shutdown evidence publication failed: ", err)
		}
		logger.Printf("service stopped instance=%s resources_settled=%t", service.InstanceID, result.ResourcesSettled)
	}()
	pipe, err := pipeName(dir)
	if err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	executableSHA, err := executableDigest(executable)
	if err != nil {
		return err
	}
	service = contracts.ServiceInfo{InstanceID: contracts.NewID(), StateDir: dir, PID: os.Getpid(), ProcessCreated: created, Status: "ready", Detached: detached, Version: resources.Version, Protocol: contracts.Protocol, ResourceDigest: resources.Digest(), ExecutableSHA256: executableSHA}
	service.UsagePolicyDigest = usagePolicy.Digest()
	service.UsagePolicyRevision = service.InstanceID
	service.UsagePolicy = &usagePolicy
	if err = store.PutUsageJournal(ctx, "effective-policy", map[string]any{"instance_id": service.InstanceID, "policy": usagePolicy, "digest": usagePolicy.Digest()}); err != nil {
		return err
	}
	if err = store.SetUsagePermit(ctx, api.CollectionPermit{Instance: service.InstanceID, PolicyDigest: usagePolicy.Digest(), Active: true}); err != nil {
		return err
	}
	defer func() {
		bounded, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		_ = store.RevokeUsagePermit(bounded, service.InstanceID)
	}()
	usageCtx, usageCancel := context.WithCancel(ctx)
	defer usageCancel()
	usageDone := make(chan struct{})
	collector := &collect.Collector{Store: store, Policy: usagePolicy, Instance: service.InstanceID}
	// Publish existing targets before advertising a ready query endpoint.
	existingRuns, readErr := store.Runs()
	if readErr != nil {
		return readErr
	}
	for _, run := range existingRuns {
		if err = store.EnsureUsage(ctx, run.ID, run.CreatedAt); err != nil {
			return err
		}
	}
	go func() { defer close(usageDone); collector.Run(usageCtx) }()
	defer func() {
		bounded, done := context.WithTimeout(context.Background(), 5*time.Second)
		_ = store.RevokeUsagePermit(bounded, service.InstanceID)
		done()
		usageCancel()
		<-usageDone
	}()
	if err := store.ReconcileStops(service.InstanceID); err != nil {
		return err
	}
	listener, err := listenPipe(pipe)
	if err != nil {
		return err
	}
	defer listener.Close()
	s := &server{endpoint: endpoint{Service: service, Pipe: pipe, Token: contracts.NewID()}, store: store, stop: make(chan string, 1), logger: logger, engine: engine}
	s.usageCancel = usageCancel
	if err := writeEndpoint(dir, s.endpoint); err != nil {
		return err
	}
	defer removeOwnEndpoint(dir, service.InstanceID)
	completeBootstrap(dir, startupID)
	logger.Printf("service ready instance=%s pid=%d detached=%t", service.InstanceID, service.PID, service.Detached)
	engine.Start()
	// Display is a best-effort projection. It owns no execution authorization.
	viewCtx, viewCancel := context.WithCancel(ctx)
	viewDone := make(chan struct{})
	go func() {
		defer close(viewDone)
		directory, viewErr := runview.Directory()
		if viewErr != nil {
			logger.Print("display export unavailable: ", viewErr)
			return
		}
		exporter := &runview.Exporter{Directory: directory, Source: runview.Source(dir), Instance: service.InstanceID, Read: engine.DisplayRuns}
		exporter.Run(viewCtx, func(err error) { logger.Print("display export unavailable: ", err) })
	}()
	defer func() {
		viewCancel()
		// A cancellation request is not proof of I/O completion. Keep the
		// original writer lease until even an unresponsive rename returns.
		select {
		case <-viewDone:
		case <-time.After(25 * time.Second):
			logger.Print("display export shutdown unverified; retaining writer lease until original I/O settles")
			<-viewDone
		}
	}()
	var activeMu sync.Mutex
	active := map[net.Conn]bool{}
	var handlers sync.WaitGroup
	acceptDone := make(chan error, 1)
	go func() {
		for {
			connection, err := listener.Accept()
			if err != nil {
				acceptDone <- err
				return
			}
			activeMu.Lock()
			active[connection] = true
			handlers.Add(1)
			activeMu.Unlock()
			go func() {
				stopID := s.handle(connection)
				connection.Close()
				activeMu.Lock()
				delete(active, connection)
				activeMu.Unlock()
				handlers.Done()
				if stopID != "" {
					s.stop <- stopID
				}
			}()
		}
	}()
	select {
	case stopID = <-s.stop:
	case <-ctx.Done():
	case err = <-acceptDone:
	}
	listener.Close()
	// Wait for Accept to finish before the final Wait: no new handlers can Add.
	if err == nil {
		<-acceptDone
	}
	activeMu.Lock()
	for connection := range active {
		connection.Close()
	}
	activeMu.Unlock()
	handlers.Wait()
	return nil
}

func (s *server) handle(connection net.Conn) string {
	_ = connection.SetDeadline(time.Now().Add(TimeoutSeconds * time.Second))
	line, err := bufio.NewReader(io.LimitReader(connection, maxWireBytes+1)).ReadBytes('\n')
	var wire wireRequest
	if err != nil || len(line) > maxWireBytes || contracts.DecodeStrict(line, &wire) != nil {
		return ""
	}
	id := ""
	if wire.Request != nil {
		id = wire.Request.RequestID
	}
	response, stop := s.dispatch(wire)
	if err := contracts.Encode(response, connection); err != nil {
		s.logger.Printf("response delivery unavailable command=%s request=%s", wire.Options.Command, id)
	}
	if stop {
		return id
	}
	return ""
}

func (s *server) dispatch(wire wireRequest) (contracts.Response, bool) {
	id := ""
	if wire.Request != nil {
		id = wire.Request.RequestID
	}
	reject := func(fault *contracts.Fault) (contracts.Response, bool) { return contracts.Rejected(id, fault), false }
	if wire.Protocol != contracts.Protocol {
		return reject(contracts.Fail(3, "unsupported_protocol", "Unsupported service protocol."))
	}
	if wire.InstanceID != s.endpoint.Service.InstanceID || wire.Token != s.endpoint.Token || wire.StateDir != s.endpoint.Service.StateDir {
		return reject(contracts.Fail(5, "stale_instance", "Service target identity does not match."))
	}
	if wire.Options.ExpectedInstanceID != "" && wire.Options.ExpectedInstanceID != s.endpoint.Service.InstanceID {
		return reject(contracts.Fail(5, "stale_instance", "Original Runner instance changed; restart SteerLine explicitly."))
	}
	if wire.Options.Command == "service stop" {
		s.mutex.Lock()
		defer s.mutex.Unlock()
	} else {
		s.mutex.RLock()
		defer s.mutex.RUnlock()
	}
	if expected := wire.Options.ExpectedBinding; expected != nil {
		if !expected.Valid() || !contracts.ServiceBindingCommand(wire.Options.Command) {
			return reject(contracts.Fail(2, "invalid_arguments", "Incomplete binding or uncovered command."))
		}
		dir, err := Normalize(expected.StateDir)
		actual := s.endpoint.Service
		if err != nil || expected.Protocol != actual.Protocol || expected.InstanceID != actual.InstanceID || dir != actual.StateDir || !strings.EqualFold(expected.ExecutableSHA256, actual.ExecutableSHA256) || expected.Version != actual.Version || !strings.EqualFold(expected.ResourceDigest, actual.ResourceDigest) {
			return reject(contracts.Fail(5, "binding_mismatch", "Original Runner service binding differs; restart SteerLine explicitly."))
		}
		if s.stopping || actual.Status != "ready" {
			return reject(contracts.Fail(5, "invalid_state", "Original Runner is not ready; no new action was admitted."))
		}
	}
	if s.stopping && contracts.ServiceBindingCommand(wire.Options.Command) {
		return reject(contracts.Fail(5, "invalid_state", "Runner is stopping; no new action was admitted."))
	}
	if wire.Request != nil {
		if wire.Request.Protocol != contracts.Protocol || !contracts.ValidRequestID(id) {
			return reject(contracts.Fail(2, "invalid_request", "Invalid request envelope."))
		}
		if invalid := contracts.ValidateInput(wire.Options.Command, wire.Request.Input); invalid != nil && wire.Options.Command != "environment-check start" && wire.Options.Command != "environment-check stop" {
			return reject(invalid)
		}
	}
	switch wire.Options.Command {
	case "observe":
		return s.engine.Observe(wire.Options.RunID, wire.Options.After, wire.Options.Limit), false
	case "run-usage", "run-usage-events", "estimate-run-cost":
		if wire.Request != nil {
			return reject(contracts.Fail(2, "invalid_request", "Usage queries do not accept input."))
		}
		return s.usageQuery(wire.Options), false
	case "service verify-takeover":
		caps, sha, failure := candidateCapabilities(context.Background(), wire.Options.Candidate)
		if failure != nil {
			return reject(failure)
		}
		items := s.engine.Takeover(caps.Recovery, caps.Workflows, caps.ResultFile.Contract)
		return contracts.Success("", finishTakeover(s.endpoint.Service.StateDir, s.endpoint.Service.InstanceID, sha, items)), false
	case "result-file":
		if wire.Request != nil {
			return reject(contracts.Fail(2, "invalid_request", "Result queries do not accept input."))
		}
		return s.engine.HandleResultFile(wire.Options.RunID), false
	case "progress-file":
		if wire.Request != nil {
			return reject(contracts.Fail(2, "invalid_request", "Progress queries do not accept input."))
		}
		return s.engine.HandleProgress(wire.Options.RunID), false
	case "artifacts discover", "artifacts collect", "artifacts list", "artifacts read", "workspace-git":
		return s.engine.HandleEvidence(wire.Options.Command, wire.Options.RunID, wire.Options.ArtifactID, wire.Request), false
	case "environment-check start", "environment-check stop", "environment-check inspect":
		return s.engine.HandleEnvironmentCheck(wire.Options.Command, wire.Options.CheckID, wire.Options.RequestID, wire.Request), false
	case "open-native-session":
		if s.stopping || wire.Request != nil || wire.Options.NativeSession == nil || wire.Options.NativeSession.RunID != wire.Options.RunID {
			return reject(contracts.Fail(5, "invalid_state", "Native viewer requires a ready service and exact original target."))
		}
		return s.engine.OpenNativeSession(*wire.Options.NativeSession), false
	case "submit", "inspect", "last-message", "events", "reply", "resume", "stop", "message", "interrupt", "close", "history":
		return s.engine.Handle(wire.Options.Command, wire.Options.RunID, wire.Options.RequestID, wire.Options.CommandID, wire.Options.After, wire.Options.Limit, wire.Options.RefreshPR, wire.Request), false
	}
	switch wire.Options.Command {
	case "service status":
		info := s.endpoint.Service
		if s.stopping {
			info.Status = "stopping"
		}
		return contracts.Success("", info), false
	case "capabilities":
		info := s.endpoint.Service
		return contracts.Success("", resources.CapabilitiesFrom(resources.CapabilitySource{Kind: "service", Version: info.Version, Protocol: info.Protocol, ExecutableSHA256: info.ExecutableSHA256, ResourceDigest: info.ResourceDigest, InstanceID: info.InstanceID})), false
	case "service stop":
		if wire.Request == nil {
			return reject(contracts.Fail(2, "invalid_request", "A stop request is required."))
		}
		response, stop, err := s.store.AcceptStop(*wire.Request, s.endpoint.Service.InstanceID)
		if err != nil {
			return reject(contracts.Fail(6, "request_outcome_unknown", "Cannot verify the shutdown receipt; inspect the original request_id."))
		}
		if stop {
			s.stopping = true
			if s.engine != nil {
				s.engine.StopObserving()
			}
			if s.usageCancel != nil {
				s.usageCancel()
			}
		}
		return response, stop
	default:
		return reject(contracts.Fail(2, "invalid_arguments", fmt.Sprintf("Unsupported service command %q.", wire.Options.Command)))
	}
}
