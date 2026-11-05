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

	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/host"
	"github.com/erranliu/SteerRunner/internal/nativecatalog"
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
			case "--expected-binding":
				var binding contracts.ExpectedServiceBinding
				if contracts.DecodeStrict([]byte(parameter), &binding) != nil || !binding.Valid() {
					return value, contracts.Fail(2, "invalid_arguments", "Expected binding requires the complete published service identity.")
				}
				value.opts.ExpectedBinding = &binding
			case "--snapshot-revision":
				revision, err := strconv.ParseUint(parameter, 10, 64)
				if err != nil || revision == 0 {
					return value, contracts.Fail(2, "invalid_arguments", "Usage revision must be a positive integer.")
				}
				value.opts.UsageRevision = revision
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
	if len(positional) == 2 && (positional[0] == "service" || positional[0] == "artifacts") {
		value.opts.Command = positional[0] + " " + positional[1]
	}
	switch value.opts.Command {
	case "progress-file", "result-file", "run-usage", "run-usage-events":
	case "artifacts discover", "artifacts collect", "artifacts list", "artifacts read", "workspace-git":
	case "native-catalog", "version", "serve", "service start", "service status", "service stop", "service verify-stopped", "capabilities", "submit", "inspect", "observe", "last-message", "open-native-session", "events", "reply", "resume", "stop", "validate", "message", "interrupt", "close", "history":
	default:
		return value, contracts.Fail(2, "invalid_arguments", "Expected a documented SteerRunner command; use --help.")
	}
	if value.protocol != contracts.Protocol {
		return value, contracts.Fail(3, "unsupported_protocol", "This executable supports "+contracts.Protocol+".")
	}
	if value.opts.Command == "native-catalog" {
		for option := range seen {
			if option != "--json" && option != "--protocol" && option != "--input" {
				return value, contracts.Fail(2, "invalid_arguments", "Native catalog does not accept service or target options.")
			}
		}
		if value.input != "-" {
			return value, contracts.Fail(2, "invalid_arguments", "Native catalog requires --input -.")
		}
		return value, nil
	}
	if value.offline && value.opts.Command != "capabilities" && value.opts.Command != "inspect" || value.opts.Command != "validate" && seen["--kind"] {
		return value, contracts.Fail(2, "invalid_arguments", "This option does not apply to this command.")
	}
	if value.offline && value.opts.Command == "capabilities" || value.opts.Command == "validate" {
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
	if value.offline && (value.opts.ExpectedBinding != nil || value.opts.ExpectedInstanceID != "") {
		return value, contracts.Fail(2, "invalid_arguments", "Offline inspect does not accept an expected service binding.")
	}
	if value.startupID != "" {
		decoded, err := hex.DecodeString(value.startupID)
		if value.opts.Command != "serve" || err != nil || len(decoded) != 16 {
			return value, contracts.Fail(2, "invalid_arguments", "Invalid internal startup identity.")
		}
	}
	if value.opts.ExpectedBinding != nil {
		if !contracts.ServiceBindingCommand(value.opts.Command) || value.opts.ExpectedInstanceID != "" && value.opts.ExpectedInstanceID != value.opts.ExpectedBinding.InstanceID {
			return value, contracts.Fail(2, "invalid_arguments", "Expected binding applies only to its published resident commands and cannot conflict with expected instance.")
		}
		value.opts.ExpectedInstanceID = value.opts.ExpectedBinding.InstanceID
	}
	if value.opts.ExpectedInstanceID != "" {
		switch value.opts.Command {
		case "version", "serve", "service start", "service status", "service stop", "service verify-stopped":
			return value, contracts.Fail(2, "invalid_arguments", "Expected instance identity applies to resident business/query commands.")
		}
	}
	mutating := writes(value.opts.Command)

	if mutating && value.input != "-" || !mutating && value.input != "" && !(value.opts.Command == "service start" && value.input == "-") {
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
		if selectors != 1 {
			return value, contracts.Fail(2, "invalid_arguments", "inspect requires exactly one identifier.")
		}
	case "progress-file", "result-file", "run-usage", "run-usage-events", "artifacts discover", "artifacts collect", "artifacts list", "artifacts read", "workspace-git", "last-message", "open-native-session", "observe", "events", "reply", "resume", "stop", "message", "interrupt", "close", "history":
		if selectors != 1 || value.opts.RunID == "" {
			return value, contracts.Fail(2, "invalid_arguments", "This command requires only run-id.")
		}
	default:
		if selectors != 0 {
			return value, contracts.Fail(2, "invalid_arguments", "This command does not accept a target identifier.")
		}
	}
	if value.opts.Command != "events" && value.opts.Command != "history" && value.opts.Command != "run-usage-events" && value.opts.Command != "observe" && (value.opts.After != "" || value.opts.Limit != 0) {
		return value, contracts.Fail(2, "invalid_arguments", "An option does not apply to this command.")
	}
	if (value.opts.Command == "events" || value.opts.Command == "history" || value.opts.Command == "run-usage-events" || value.opts.Command == "observe") && value.opts.Limit == 0 {
		value.opts.Limit = 200
	}
	if seen["--snapshot-revision"] && (value.opts.Command != "run-usage" && value.opts.Command != "run-usage-events") {
		return value, contracts.Fail(2, "invalid_arguments", "Usage options do not apply to this command.")
	}
	if value.opts.Command == "run-usage-events" && !seen["--snapshot-revision"] {
		return value, contracts.Fail(2, "invalid_arguments", "Usage events require an explicit snapshot revision.")
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
	case "submit", "service stop", "reply", "resume", "stop", "message", "interrupt", "close":
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
	if options.opts.Command == "native-catalog" {
		raw, err := io.ReadAll(io.LimitReader(input, contracts.NativeCatalogInputLimit+1))
		if err != nil {
			return reject(contracts.Fail(1, "internal_error", "Cannot read catalog input."))
		}
		query, fault := contracts.ReadNativeCatalog(raw)
		if fault != nil {
			return reject(fault)
		}
		executable, err := os.Executable()
		if err != nil {
			return reject(contracts.Fail(1, "internal_error", "Cannot identify catalog executable."))
		}
		file, err := os.Open(executable)
		if err != nil {
			return reject(contracts.Fail(1, "internal_error", "Cannot identify catalog executable."))
		}
		digest := sha256.New()
		_, err = io.Copy(digest, file)
		file.Close()
		if err != nil {
			return reject(contracts.Fail(1, "internal_error", "Cannot verify catalog executable."))
		}
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		result, fault := nativecatalog.Query(ctx, query, contracts.NativeCatalogSource{Version: resources.Version, ExecutableSHA256: hex.EncodeToString(digest.Sum(nil)), ResourceDigest: resources.Digest()})
		if fault != nil {
			return reject(fault)
		}
		response := contracts.Success("", result)
		encoded, err := json.Marshal(response)
		if err != nil || len(encoded)+1 > contracts.NativeCatalogOutputLimit {
			return reject(contracts.Fail(4, "native_catalog_unavailable", "Catalog response exceeds its limit."))
		}
		return finish(response, 0)
	}
	if options.opts.Command == "version" {
		return finish(contracts.Success("", resources.VersionInfo()), 0)
	}
	if options.opts.Command == "help" {
		return finish(contracts.Success("", map[string]any{"usage": "steerrunner <command> --state-dir <absolute-path> --json", "commands": []string{"native-catalog", "progress-file", "result-file", "run-usage", "run-usage-events", "version", "service start", "service status", "service stop", "service verify-stopped", "capabilities", "submit", "inspect", "last-message", "open-native-session", "observe", "events", "reply", "resume", "stop", "validate", "message", "interrupt", "close", "history", "artifacts discover", "artifacts collect", "artifacts list", "artifacts read", "workspace-git"}, "input": "Write commands consume a UTF-8 JSON envelope from --input -. validate consumes a bare object. service start optionally consumes a bare pending_stops object.", "static": "capabilities --offline; validate --kind run-configuration|run --input -", "run_evidence": "artifacts discover|list --run-id R; artifacts collect --run-id R --input -; artifacts read --run-id R --artifact-id A; workspace-git --run-id R. Producer writes .steerrunner/artifacts/<relative_path> in original workspace. Max file 1 MiB. Read is base64; schema/limits in capabilities.run_evidence.", "instance_binding": "--expected-instance-id <id> fences resident business/query commands and disables implicit service startup."}), 0)
