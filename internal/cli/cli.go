// Package cli is a short-lived client. It never opens the service database.
package cli

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/host"
	"github.com/erranliu/SteerRunner/internal/resources"
)

type arguments struct {
	opts      host.Options
	dir       string
	protocol  string
	input     string
	startupID string
	help      bool
}

func parse(argv []string) (arguments, *contracts.Fault) {
	value := arguments{protocol: contracts.Protocol}
	var positional []string
	seen := map[string]bool{}
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		if strings.HasPrefix(arg, "-") {
			name, _, _ := strings.Cut(arg, "=")
			if seen[name] {
				return value, contracts.Fail(2, "invalid_arguments", "An option was supplied more than once.")
			}
			seen[name] = true
		}
		switch arg {
		case "--json":
		case "--help", "-h":
			value.help = true
		case "--refresh-pr":
			value.opts.RefreshPR = true
		default:
			if !strings.HasPrefix(arg, "-") {
				positional = append(positional, arg)
				continue
			}
			name, parameter, supplied := strings.Cut(arg, "=")
			if !supplied {
				if i+1 >= len(argv) {
					return value, contracts.Fail(2, "invalid_arguments", "An option value is missing.")
				}
				i++
				parameter = argv[i]
			}
			switch name {
			case "--state-dir":
				value.dir = parameter
			case "--protocol":
				value.protocol = parameter
			case "--input":
				value.input = parameter
			case "--run-id":
				value.opts.RunID = parameter
			case "--request-id":
				value.opts.RequestID = parameter
			case "--command-id":
				value.opts.CommandID = parameter
			case "--after":
				value.opts.After = parameter
			case "--limit":
				limit, err := strconv.Atoi(parameter)
				if err != nil || limit < 1 || limit > 500 {
					return value, contracts.Fail(2, "invalid_arguments", "limit must be between 1 and 500.")
				}
				value.opts.Limit = limit
			case "--startup-id":
				value.startupID = parameter
			default:
				return value, contracts.Fail(2, "invalid_arguments", "An unknown option was supplied.")
			}
		}
	}
	if value.help {
		value.opts.Command = "help"
		return value, nil
	}
	if len(positional) == 1 {
		value.opts.Command = positional[0]
	}
	if len(positional) == 2 && positional[0] == "service" {
		value.opts.Command = "service " + positional[1]
	}
	switch value.opts.Command {
	case "version", "serve", "service start", "service status", "service stop", "capabilities", "submit", "inspect", "events", "reply", "resume", "stop":
	default:
		return value, contracts.Fail(2, "invalid_arguments", "Expected a documented SteerRunner command; use --help.")
	}
	if value.protocol != contracts.Protocol {
		return value, contracts.Fail(3, "unsupported_protocol", "This executable supports "+contracts.Protocol+".")
	}
	if value.startupID != "" {
		decoded, err := hex.DecodeString(value.startupID)
		if value.opts.Command != "serve" || err != nil || len(decoded) != 16 {
			return value, contracts.Fail(2, "invalid_arguments", "Invalid internal startup identity.")
		}
	}
	mutating := writes(value.opts.Command)
	if mutating && value.input != "-" || !mutating && value.input != "" {
		return value, contracts.Fail(2, "invalid_arguments", "Write commands require --input -; other commands do not accept input.")
	}
	selectors := 0
	for _, id := range []string{value.opts.RunID, value.opts.RequestID, value.opts.CommandID} {
		if id != "" {
			selectors++
		}
	}
	switch value.opts.Command {
	case "inspect":
		if selectors != 1 || value.opts.RefreshPR && value.opts.RunID == "" {
			return value, contracts.Fail(2, "invalid_arguments", "inspect requires exactly one identifier; refresh-pr requires run-id.")
		}
	case "events", "reply", "resume", "stop":
		if selectors != 1 || value.opts.RunID == "" {
			return value, contracts.Fail(2, "invalid_arguments", "This command requires only run-id.")
		}
	default:
		if selectors != 0 {
			return value, contracts.Fail(2, "invalid_arguments", "This command does not accept a target identifier.")
		}
	}
	if value.opts.Command != "events" && (value.opts.After != "" || value.opts.Limit != 0) || value.opts.Command != "inspect" && value.opts.RefreshPR {
		return value, contracts.Fail(2, "invalid_arguments", "An option does not apply to this command.")
	}
	if value.opts.Command == "events" && value.opts.Limit == 0 {
		value.opts.Limit = 200
	}
	return value, nil
}

func writes(command string) bool {
	switch command {
	case "submit", "service stop", "reply", "resume", "stop":
		return true
	default:
		return false
	}
}

func Run(argv []string, input io.Reader, output, diagnostics io.Writer) int {
	options, parseErr := parse(argv)
	id := ""
	finish := func(response contracts.Response, exit int) int {
		if err := contracts.Encode(response, output); err != nil {
			fmt.Fprintln(diagnostics, "Cannot write the CLI response.")
			return 1
		}
		return exit
	}
	reject := func(fault *contracts.Fault) int { return finish(contracts.Rejected(id, fault), fault.Exit) }
	if parseErr != nil {
		return reject(parseErr)
	}
	if options.opts.Command == "version" {
		return finish(contracts.Success("", resources.VersionInfo()), 0)
	}
	if options.opts.Command == "help" {
		return finish(contracts.Success("", map[string]any{"usage": "steerrunner <command> --state-dir <absolute-path> --json", "commands": []string{"version", "service start", "service status", "service stop", "capabilities", "submit", "inspect", "events", "reply", "resume", "stop"}, "input": "Write commands consume a UTF-8 JSON envelope from --input -."}), 0)
	}
	var request *contracts.Request
	if writes(options.opts.Command) {
		value, requestErr := contracts.ReadRequest(input)
		id = value.RequestID
		if requestErr != nil {
			return reject(requestErr)
		}
		if invalid := contracts.ValidateInput(options.opts.Command, value.Input); invalid != nil {
			return reject(invalid)
		}
		request = &value
	}
	dir, err := host.Normalize(options.dir)
	if err != nil {
		return reject(contracts.Fail(2, "invalid_arguments", "Cannot resolve an absolute state-dir."))
	}
	if options.opts.Command == "serve" {
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		if err := host.Serve(ctx, dir, options.startupID); err != nil {
			fault := host.PublicFault(err)
			host.RecordStartupFailure(dir, options.startupID, fault)
			fmt.Fprintln(diagnostics, fault.Error())
			return fault.Exit
		}
		return 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), host.TimeoutSeconds*time.Second)
	defer cancel()
	if options.opts.Command == "service start" || options.opts.Command == "submit" {
		executable, err := os.Executable()
		if err != nil {
			return reject(contracts.Fail(4, "service_start_failed", "Cannot locate this executable."))
		}
		service, startErr := host.Ensure(ctx, dir, executable)
		if startErr != nil {
			return reject(startErr)
		}
		if options.opts.Command == "service start" {
			return finish(contracts.Success("", service), 0)
		}
	}
	if options.opts.Command == "service status" {
		service, statusErr := host.Status(ctx, dir)
		if statusErr != nil {
			return reject(statusErr)
		}
		return finish(contracts.Success("", service), 0)
	}
	response, callErr := host.Call(ctx, dir, options.opts, request)
	if callErr != nil {
		return reject(callErr)
	}
	return finish(response, contracts.ExitCode(response))
}
