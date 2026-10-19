// Package cli is a short-lived client. Offline verification is delegated to
// the host under the writer lease; ordinary queries never open storage.
package cli

import (
	"context"
	"crypto/sha256"
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
	offline   bool
	kind      string
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
		case "--offline":
			value.offline = true
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
			case "--kind":
				value.kind = parameter
			case "--expected-instance-id":
				if !contracts.ValidRequestID(parameter) {
					return value, contracts.Fail(2, "invalid_arguments", "Expected instance identity must be nonempty plain text.")
				}
				value.opts.ExpectedInstanceID = parameter
			case "--state-dir":
				value.dir = parameter
			case "--protocol":
				value.protocol = parameter
			case "--input":
				value.input = parameter
			case "--check-id":
				value.opts.CheckID = parameter
			case "--run-id":
				value.opts.RunID = parameter
			case "--artifact-id":
				value.opts.ArtifactID = parameter
			case "--attempt-id", "--session-id", "--generation":
				if value.opts.NativeSession == nil {
					value.opts.NativeSession = &contracts.NativeSessionTarget{}
				}
				switch name {
				case "--attempt-id":
					value.opts.NativeSession.AttemptID = parameter
				case "--session-id":
					value.opts.NativeSession.SessionID = parameter
				case "--generation":
					value.opts.NativeSession.Generation = parameter
				}
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
	if len(positional) == 2 && (positional[0] == "service" || positional[0] == "environment-check" || positional[0] == "artifacts") {
		value.opts.Command = positional[0] + " " + positional[1]
	}
	switch value.opts.Command {
	case "artifacts discover", "artifacts collect", "artifacts list", "artifacts read", "workspace-git":
	case "version", "serve", "service start", "service status", "service stop", "service verify-stopped", "capabilities", "environment-check start", "environment-check stop", "environment-check inspect", "submit", "inspect", "last-message", "open-native-session", "events", "reply", "resume", "stop", "validate", "message", "interrupt", "close", "history":
	default:
		return value, contracts.Fail(2, "invalid_arguments", "Expected a documented SteerRunner command; use --help.")
	}
	if value.protocol != contracts.Protocol {
		return value, contracts.Fail(3, "unsupported_protocol", "This executable supports "+contracts.Protocol+".")
	}
	if value.offline && value.opts.Command != "capabilities" || value.opts.Command != "validate" && seen["--kind"] {
		return value, contracts.Fail(2, "invalid_arguments", "This option does not apply to this command.")
	}
	if value.offline || value.opts.Command == "validate" {
		for option := range seen {
			if option != "--json" && option != "--protocol" && option != "--offline" && !(value.opts.Command == "validate" && (option == "--kind" || option == "--input")) {
				return value, contracts.Fail(2, "invalid_arguments", "Static commands do not accept service or target options.")
			}
		}
		if value.opts.Command == "validate" && (!resources.ValidationKind(value.kind) || value.input != "-") {
			return value, contracts.Fail(2, "invalid_arguments", "validate requires a supported --kind and --input -.")
		}
		return value, nil
	}
	if value.startupID != "" {
		decoded, err := hex.DecodeString(value.startupID)
		if value.opts.Command != "serve" || err != nil || len(decoded) != 16 {
			return value, contracts.Fail(2, "invalid_arguments", "Invalid internal startup identity.")
		}
	}
	if value.opts.ExpectedInstanceID != "" {
		switch value.opts.Command {
		case "version", "serve", "service start", "service status", "service stop", "service verify-stopped":
			return value, contracts.Fail(2, "invalid_arguments", "Expected instance identity applies to resident business/query commands.")
		}
	}
	mutating := writes(value.opts.Command)
	if mutating && value.input != "-" || !mutating && value.input != "" {
		return value, contracts.Fail(2, "invalid_arguments", "Write commands require --input -; other commands do not accept input.")
	}
	selectors := 0
	for _, id := range []string{value.opts.RunID, value.opts.RequestID, value.opts.CommandID, value.opts.CheckID} {
		if id != "" {
			selectors++
		}
	}
	switch value.opts.Command {
	case "environment-check inspect":
		if selectors != 1 || (value.opts.CheckID == "" && value.opts.RequestID == "") {
			return value, contracts.Fail(2, "invalid_arguments", "environment-check inspect requires exactly one check-id or request-id.")
		}
	case "inspect":
		if selectors != 1 || value.opts.CheckID != "" || value.opts.RefreshPR && value.opts.RunID == "" {
			return value, contracts.Fail(2, "invalid_arguments", "inspect requires exactly one identifier; refresh-pr requires run-id.")
		}
	case "artifacts discover", "artifacts collect", "artifacts list", "artifacts read", "workspace-git", "last-message", "open-native-session", "events", "reply", "resume", "stop", "message", "interrupt", "close", "history":
		if selectors != 1 || value.opts.RunID == "" {
			return value, contracts.Fail(2, "invalid_arguments", "This command requires only run-id.")
		}
	default:
		if selectors != 0 {
			return value, contracts.Fail(2, "invalid_arguments", "This command does not accept a target identifier.")
		}
	}
	if value.opts.Command != "events" && value.opts.Command != "history" && (value.opts.After != "" || value.opts.Limit != 0) || value.opts.Command != "inspect" && value.opts.RefreshPR {
		return value, contracts.Fail(2, "invalid_arguments", "An option does not apply to this command.")
	}
	if (value.opts.Command == "events" || value.opts.Command == "history") && value.opts.Limit == 0 {
		value.opts.Limit = 200
	}
	if value.opts.Command == "artifacts read" {
		if !contracts.ValidRequestID(value.opts.ArtifactID) {
			return value, contracts.Fail(2, "invalid_arguments", "artifacts read requires --artifact-id.")
		}
	} else if seen["--artifact-id"] {
		return value, contracts.Fail(2, "invalid_arguments", "artifact-id applies only to artifacts read.")
	}
	if value.opts.Command == "open-native-session" {
		target := value.opts.NativeSession
		if target == nil || target.AttemptID == "" || target.SessionID == "" || target.Generation == "" {
			return value, contracts.Fail(2, "invalid_arguments", "Native viewer requires run-id, attempt-id, session-id and generation.")
		}
		target.RunID = value.opts.RunID
	} else if value.opts.NativeSession != nil {
		return value, contracts.Fail(2, "invalid_arguments", "Native session options apply only to open-native-session.")
	}
	return value, nil
}

func writes(command string) bool {
	switch command {
	case "artifacts collect":
		return true
	case "environment-check start", "environment-check stop", "submit", "service stop", "reply", "resume", "stop", "message", "interrupt", "close":
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
		return finish(contracts.Success("", map[string]any{"usage": "steerrunner <command> --state-dir <absolute-path> --json", "commands": []string{"version", "service start", "service status", "service stop", "service verify-stopped", "capabilities", "environment-check start", "environment-check stop", "environment-check inspect", "submit", "inspect", "last-message", "open-native-session", "events", "reply", "resume", "stop", "validate", "message", "interrupt", "close", "history", "artifacts discover", "artifacts collect", "artifacts list", "artifacts read", "workspace-git"}, "input": "Write commands consume a UTF-8 JSON envelope from --input -. validate consumes a bare object, without a request envelope.", "static": "capabilities --offline; validate --kind run-configuration|run|environment-check --input -", "run_evidence": "artifacts discover|list --run-id R; artifacts collect --run-id R --input -; artifacts read --run-id R --artifact-id A; workspace-git --run-id R. Producer writes .steerrunner/artifacts/<relative_path> in original workspace. Max file 1 MiB. Read is base64; schema/limits in capabilities.run_evidence.", "instance_binding": "--expected-instance-id <id> fences resident business/query commands and disables implicit service startup."}), 0)
	}
	if options.offline {
		executable, err := os.Executable()
		if err != nil {
			return reject(contracts.Fail(1, "internal_error", "Cannot identify this executable."))
		}
		file, err := os.Open(executable)
		if err != nil {
			return reject(contracts.Fail(1, "internal_error", "Cannot verify this executable."))
		}
		digest := sha256.New()
		_, err = io.Copy(digest, file)
		file.Close()
		if err != nil {
			return reject(contracts.Fail(1, "internal_error", "Cannot verify this executable."))
		}
		return finish(contracts.Success("", resources.CapabilitiesFrom(resources.CapabilitySource{Kind: "executable", Version: resources.Version, Protocol: contracts.Protocol, ExecutableSHA256: hex.EncodeToString(digest.Sum(nil)), ResourceDigest: resources.Digest()})), 0)
	}
	if options.opts.Command == "validate" {
		data, err := io.ReadAll(io.LimitReader(input, contracts.MaxRequestBytes+1))
		if err != nil {
			return reject(contracts.Fail(1, "internal_error", "Cannot read the validation input."))
		}
		if len(data) > contracts.MaxRequestBytes {
			result := resources.ValidationResult{Kind: options.kind, Valid: false, Issues: []resources.Issue{{Path: "", Code: "invalid_value", Message: "Input exceeds the public size limit."}}, Version: resources.Version, ResourceDigest: resources.Digest()}
			response := contracts.Rejected("", contracts.Fail(2, "invalid_input", "Validation input exceeds the public size limit."))
			response.Data = result
			return finish(response, 2)
		}
		result, fault := resources.Validate(options.kind, data)
		if fault != nil {
			response := contracts.Rejected("", fault)
			if fault.Exit == 2 || fault.Exit == 3 {
				response.Data = result
			}
			return finish(response, fault.Exit)
		}
		return finish(contracts.Success("", result), 0)
	}
	var request *contracts.Request
	if writes(options.opts.Command) {
		value, requestErr := contracts.ReadRequest(input)
		id = value.RequestID
		if requestErr != nil {
			return reject(requestErr)
		}
		if invalid := contracts.ValidateInput(options.opts.Command, value.Input); invalid != nil && options.opts.Command != "environment-check start" && options.opts.Command != "environment-check stop" {
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
	if options.opts.Command == "service start" || options.opts.ExpectedInstanceID == "" && (options.opts.Command == "submit" || options.opts.Command == "environment-check start") {
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
	if options.opts.Command == "service verify-stopped" {
		result, verifyErr := host.VerifyStopped(ctx, dir)
		if verifyErr != nil {
			return reject(verifyErr)
		}
		return finish(contracts.Success("", result), 0)
	}
	response, callErr := host.Call(ctx, dir, options.opts, request)
	if callErr != nil {
		return reject(callErr)
	}
	return finish(response, contracts.ExitCode(response))
}
