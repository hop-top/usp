package api

import (
	"context"
	"crypto/rand"
	"fmt"

	"hop.top/kit/go/core/xdg"
	"hop.top/usp/internal/sessionutil"
	"hop.top/usp/lineage"
	"hop.top/usp/session"
)

type ResumeSessionRequest struct {
	ID          string
	TargetCLI   string
	ProjectCWD  string
	LineagePath string
}

type ResumeSessionResult struct {
	USPID        string
	SourceCLI    string
	TargetCLI    string
	TargetNative string
	Command      []string
}

func (s *Service) ResumeSession(ctx context.Context, req ResumeSessionRequest) (*ResumeSessionResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var (
		sourceSession *session.Session
		sourceCLI     string
		sourceAdapter session.SessionAdapter
	)
	if req.ID != "" {
		var err error
		sourceSession, sourceCLI, sourceAdapter, err = sessionutil.ResolveSessionID(
			req.ID, s.adapters, AdapterOrder(req.ID))
		if err != nil {
			return nil, err
		}
	} else {
		all := sessionutil.CollectSessions(s.adapters, req.ProjectCWD)
		all = sessionutil.SortAndLimit(all, 1)
		if len(all) == 0 {
			return nil, fmt.Errorf("no sessions found for %s", req.ProjectCWD)
		}
		sourceSession = &all[0]
		sourceCLI = string(sourceSession.CLI)
		sourceAdapter = s.adapters[sourceCLI]
	}

	if req.TargetCLI == "" {
		return nil, fmt.Errorf(
			"specify target CLI with --cli (source: %s)", sourceCLI)
	}
	targetAdapter, ok := s.adapters[req.TargetCLI]
	if !ok {
		return nil, fmt.Errorf("unknown CLI %q", req.TargetCLI)
	}
	target, ok := targetAdapter.(session.ResumeAdapter)
	if !ok {
		return nil, fmt.Errorf(
			"%q does not support resume (ResumeAdapter not implemented)",
			req.TargetCLI,
		)
	}

	ch, err := sourceAdapter.StreamTurns(sourceSession.NativeID)
	if err != nil {
		return nil, fmt.Errorf("stream turns: %w", err)
	}
	var turns []session.Turn
	for turn := range ch {
		turns = append(turns, turn)
	}

	nativeID, err := target.InjectSession(req.ProjectCWD, turns)
	if err != nil {
		return nil, fmt.Errorf("inject session: %w", err)
	}

	lineagePath := req.LineagePath
	if lineagePath == "" {
		lineagePath, err = xdg.StateFile("usp", "sessions.db")
		if err != nil {
			return nil, fmt.Errorf("lineage path: %w", err)
		}
	}
	store, err := lineage.Open(lineagePath)
	if err != nil {
		return nil, fmt.Errorf("lineage store: %w", err)
	}
	defer func() { _ = store.Close() }()

	uspID := generateLineageID()
	if err := store.CreateSession(uspID, req.ProjectCWD); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	if err := store.AddSegment(
		uspID, sourceCLI, sourceSession.NativeID, len(turns),
	); err != nil {
		return nil, fmt.Errorf("add source segment: %w", err)
	}
	if err := store.AddSegment(
		uspID, req.TargetCLI, nativeID, 0,
	); err != nil {
		return nil, fmt.Errorf("add target segment: %w", err)
	}

	return &ResumeSessionResult{
		USPID:        uspID,
		SourceCLI:    sourceCLI,
		TargetCLI:    req.TargetCLI,
		TargetNative: nativeID,
		Command:      target.ResumeCmd(nativeID),
	}, nil
}

func generateLineageID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf(
		"%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16],
	)
}
