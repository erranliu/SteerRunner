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
	"sync"
	"time"

	"github.com/erranliu/SteerRunner/internal/application"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/resources"
	"github.com/erranliu/SteerRunner/internal/storage"
)

type server struct {
	engine   *application.Engine
	endpoint endpoint
	store    *storage.Store
	mutex    sync.Mutex // command acceptance and service state are serialized
	stopping bool
	stop     chan string
	logger   *log.Logger
}

func Serve(ctx context.Context, dir, startupID string) error {
	created, detached, err := launchFacts(dir, startupID)
	if err != nil {
		return err
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
	output, err := os.OpenFile(filepath.Join(dir, "logs", "service.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer output.Close()
	logger := log.New(output, "", log.Ldate|log.Ltime|log.LUTC)
	store, err := storage.Open(dir)
	if err != nil {
		logger.Print("startup rejected: ", err)
		return err
	}
	defer store.Close()
	engine, err := application.New(ctx, dir, store, logger)
	if err != nil {
		return err
	}
	defer func() {
		shutdown, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		if err := engine.Shutdown(shutdown); err != nil {
			logger.Print("execution shutdown remains unverified")
		}
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
	service := contracts.ServiceInfo{InstanceID: contracts.NewID(), StateDir: dir, PID: os.Getpid(), ProcessCreated: created, Status: "ready", Detached: detached, Version: resources.Version, Protocol: contracts.Protocol, ResourceDigest: resources.Digest(), ExecutableSHA256: executableSHA}
	if err := store.ReconcileStops(service.InstanceID); err != nil {
		return err
	}
	listener, err := listenPipe(pipe)
	if err != nil {
		return err
	}
	defer listener.Close()
	s := &server{endpoint: endpoint{Service: service, Pipe: pipe, Token: contracts.NewID()}, store: store, stop: make(chan string, 1), logger: logger, engine: engine}
	if err := writeEndpoint(dir, s.endpoint); err != nil {
		return err
	}
	defer removeOwnEndpoint(dir, service.InstanceID)
	completeBootstrap(dir, startupID)
	logger.Printf("service ready instance=%s pid=%d detached=%t", service.InstanceID, service.PID, service.Detached)
	engine.Start()
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
	stopID := ""
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
	if stopID != "" {
		if completeErr := store.CompleteStop(stopID); completeErr != nil {
			logger.Print("shutdown receipt remains pending: ", completeErr)
		}
	}
	logger.Printf("service stopped instance=%s", service.InstanceID)
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
	if wire.Request != nil {
		if wire.Request.Protocol != contracts.Protocol || !contracts.ValidRequestID(id) {
			return reject(contracts.Fail(2, "invalid_request", "Invalid request envelope."))
		}
		if invalid := contracts.ValidateInput(wire.Options.Command, wire.Request.Input); invalid != nil {
			return reject(invalid)
		}
	}
	switch wire.Options.Command {
	case "submit", "inspect", "last-message", "events", "reply", "resume", "stop":
		return s.engine.Handle(wire.Options.Command, wire.Options.RunID, wire.Options.RequestID, wire.Options.CommandID, wire.Options.After, wire.Options.Limit, wire.Options.RefreshPR, wire.Request), false
	}
	s.mutex.Lock()
	defer s.mutex.Unlock()
	switch wire.Options.Command {
	case "service status":
		info := s.endpoint.Service
		if s.stopping {
			info.Status = "stopping"
		}
		return contracts.Success("", info), false
	case "capabilities":
		return contracts.Success("", resources.Capabilities()), false
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
		}
		return response, stop
	default:
		return reject(contracts.Fail(2, "invalid_arguments", fmt.Sprintf("Unsupported service command %q.", wire.Options.Command)))
	}
}
