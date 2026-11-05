// Package nativecatalog runs bounded native metadata queries without a service.
package nativecatalog

import (
	"context"
	"errors"
	"github.com/erranliu/SteerRunner/internal/contracts"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	"github.com/erranliu/SteerRunner/internal/metadataprocess"
	"runtime"
	"time"
)

func Query(ctx context.Context, in contracts.NativeCatalogRequest, source contracts.NativeCatalogSource) (contracts.NativeCatalogResult, *contracts.Fault) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out := contracts.NativeCatalogResult{Contract: contracts.NativeCatalogContract, Kind: in.Kind, Adapter: in.Adapter, Target: in.Target, Agent: in.Agent, Source: source, ContextUnverified: true, State: "ready", Complete: true, Origin: "unknown"}
	err := query(ctx, in, &out)
	if err != nil {
		if errors.Is(err, orca.ErrCatalogUnsupported) {
			return out, contracts.Fail(3, "unsupported_capability", "Native directory protocol or agent is unsupported.")
		}
		if errors.Is(err, metadataprocess.ErrCleanup) {
			return out, contracts.Fail(4, "native_catalog_cleanup_unverified", "Native directory query cleanup remains unverified.")
		}
		if ctx.Err() != nil {
			return out, contracts.Fail(4, "native_catalog_timeout", "Native directory query exceeded its deadline or was canceled.")
		}
		return out, contracts.Fail(4, "native_catalog_unavailable", "Native directory query is unavailable or incomplete.")
	}
	out.ObservedAt = time.Now().UTC()
	return out, nil
}

func query(ctx context.Context, in contracts.NativeCatalogRequest, out *contracts.NativeCatalogResult) error {
	if in.Kind == "environments" {
		rows, err := orca.CatalogEnvironments(ctx)
		if err != nil {
			return err
		}
		environments := make([]contracts.NativeEnvironment, 0, len(rows))
		for _, row := range rows {
			environments = append(environments, contracts.NativeEnvironment{ID: row.ID, Name: row.Name})
		}
		out.Environments = &environments
		out.Origin = "registered"
		return nil
	}
	if in.Adapter == "codex-app-server" {
		program := in.Executable
		if program == "" {
			program = "codex"
			if runtime.GOOS == "windows" {
				program = "codex.exe"
			}
		}
		executable, err := metadataprocess.Executable(program)
		if err != nil {
			return err
		}
		models, version, err := codexModels(ctx, executable)
		if err != nil {
			return err
		}
		out.Models = &models
		out.Source.NativeVersion = version
		out.Origin = "probe"
		return nil
	}
	result, err := orca.QueryCatalog(ctx, in.Target.Environment, in.Kind, in.Agent)
	if err != nil {
		return err
	}
	out.Source.NativeVersion = result.Version
	out.Source.RuntimeID = result.RuntimeID
	out.Source.Protocol = result.Protocol
	out.Origin = result.Origin
	out.NativeFetchedAt = result.FetchedAt
	out.ListingInProgress = result.Listing
	if result.Listing {
		out.State = "loading"
		out.Complete = false
	}
	if in.Kind == "agents" {
		agents := make([]contracts.NativeAgent, 0, len(result.Agents))
		for _, id := range result.Agents {
			agents = append(agents, contracts.NativeAgent{ID: id, Directory: "registered"})
		}
		out.Agents = &agents
	} else {
		models := make([]contracts.NativeModel, 0, len(result.Models))
		for _, row := range result.Models {
			efforts := make([]contracts.NativeEffort, 0, len(row.Efforts))
			for _, e := range row.Efforts {
				efforts = append(efforts, contracts.NativeEffort{Value: e.Value, Label: e.Label})
			}
			models = append(models, contracts.NativeModel{ID: row.ID, Label: row.Label, Provider: row.Provider, Default: row.Default, DefaultEffort: row.DefaultEffort, Efforts: efforts})
		}
		out.Models = &models
	}
	return nil
}
