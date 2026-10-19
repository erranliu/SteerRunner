// steerrunner-agent is installed on the execution host beside its trusted gh
// login. Its configuration and stdout contain no credentials or capabilities.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	"github.com/erranliu/SteerRunner/internal/execution/terminaltools"
	"os"
	"os/exec"
	"time"
)

func main() {
	if len(os.Args) == 3 && os.Args[1] == "--native-control" {
		var setup codex.RemoteControlSetup
		raw, err := base64.RawURLEncoding.DecodeString(os.Args[2])
		if err == nil {
			err = contracts.DecodeStrict(raw, &setup)
		}
		if err == nil {
			err = codex.ServeRemoteControl(context.Background(), setup, os.Stdin, os.Stdout)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "native control unavailable")
			os.Exit(1)
		}
		return
	}
	if len(os.Args) == 2 && os.Args[1] == "--terminal-metadata" {
		metadata, err := orca.TerminalRuntimeMetadata()
		if err != nil {
			fmt.Fprintln(os.Stderr, "terminal runtime metadata unavailable")
			os.Exit(1)
		}
		raw, _ := json.Marshal(metadata)
		fmt.Println(string(raw))
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "--terminal-configuration" {
		var config orca.TerminalConfig
		raw, err := base64.RawURLEncoding.DecodeString(os.Args[2])
		if err == nil {
			err = contracts.DecodeStrict(raw, &config)
		}
		if err == nil {
			err = orca.RunTerminalAgent(context.Background(), config, os.Stdin, os.Stdout, os.Stderr)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "Native terminal preparation/execution failed:", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "--terminal-tool" {
		var config terminaltools.Config
		raw, err := base64.RawURLEncoding.DecodeString(os.Args[2])
		if err == nil {
			err = contracts.DecodeStrict(raw, &config)
		}
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			err = terminaltools.Run(ctx, config, os.Stdin, os.Stdout, os.Stderr)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "terminal tool failed:", err)
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				os.Exit(1)
			}
			os.Exit(2)
		}
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "--authentication-stop" {
		var query codex.RemoteAuthenticationQuery
		raw, err := base64.RawURLEncoding.DecodeString(os.Args[2])
		if err == nil {
			err = contracts.DecodeStrict(raw, &query)
		}
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()
			proof, e := codex.RevokeRemoteAuthentication(ctx, query)
			err = e
			if err == nil {
				body, _ := json.Marshal(proof)
				fmt.Println(codex.RemoteAuthenticationPrefix + base64.RawURLEncoding.EncodeToString(body))
				return
			}
		}
		fmt.Fprintln(os.Stderr, "remote authentication stop unverified")
		os.Exit(1)
	}
	if len(os.Args) == 3 && os.Args[1] == "--original-turn" {
		var query codex.RemoteTurnQuery
		raw, err := base64.RawURLEncoding.DecodeString(os.Args[2])
		if err == nil {
			err = contracts.DecodeStrict(raw, &query)
		}
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
			defer cancel()
			value, readErr := codex.InspectRemoteTurn(ctx, query)
			err = readErr
			if value.ReaderWritersAbsent {
				body, _ := json.Marshal(value)
				fmt.Println(codex.RemoteTurnPrefix + base64.RawURLEncoding.EncodeToString(body))
			}
			if err == nil && value.ReaderWritersAbsent {
				return
			}
		}
		fmt.Fprintln(os.Stderr, "remote original turn inspection failed")
		os.Exit(1)
	}
	if len(os.Args) == 3 && os.Args[1] == "--environment-session" {
		var setup codex.RemoteCheckSetup
		raw, err := base64.RawURLEncoding.DecodeString(os.Args[2])
		if err == nil {
			err = contracts.DecodeStrict(raw, &setup)
		}
		startup := "remote_check_configuration_invalid"
		if err == nil {
			err = codex.ServeRemoteCheck(context.Background(), setup, os.Stdin, os.Stdout)
			startup = codex.RemoteCheckStartupDiagnostic(err)
		}
		if err != nil {
			if startup != "" {
				fmt.Fprintln(os.Stdout, codex.RemoteCheckStartupFailurePrefix+startup)
			} else {
				fmt.Fprintln(os.Stderr, "remote environment session failed:", codex.EnvironmentDiagnostic(err))
			}
			os.Exit(1)
		}
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "--audit" {
		var input struct{ Workspace, Session string }
		raw, err := base64.RawURLEncoding.DecodeString(os.Args[2])
		if err == nil {
			err = contracts.DecodeStrict(raw, &input)
		}
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
			defer cancel()
			result, auditErr := codex.AuditRemoteAgent(ctx, input.Workspace, input.Session)
			err = auditErr
			if err == nil {
				body, _ := json.Marshal(result)
				fmt.Println("SR_REMOTE_AUDIT:" + base64.RawURLEncoding.EncodeToString(body))
				return
			}
		}
		fmt.Fprintln(os.Stderr, "remote native audit failed:", err)
		os.Exit(1)
	}
	if len(os.Args) != 3 || os.Args[1] != "--configuration" {
		fmt.Fprintln(os.Stderr, "usage: steerrunner-agent --configuration <public-base64url-json>")
		os.Exit(2)
	}
	raw, err := base64.RawURLEncoding.DecodeString(os.Args[2])
	var c codex.RemoteAgentConfig
	if err == nil {
		err = contracts.DecodeStrict(raw, &c)
	}
	if err == nil {
		err = codex.RunRemoteAgent(context.Background(), c, os.Stdin, os.Stdout, os.Stderr)
	}
	if err != nil {
		diagnostic := codex.EnvironmentDiagnostic(err)
		if diagnostic == "preparation_unverified" {
			diagnostic = "remote_agent_execution_failed"
		}
		fmt.Fprintln(os.Stdout, codex.RemoteFailurePrefix+diagnostic)
		fmt.Fprintln(os.Stderr, "remote agent preparation/execution failed:", diagnostic)
		os.Exit(1)
	}
}
