package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
	"hop.top/kit/go/console/wizard"
	"hop.top/usp/internal/api"
)

const sessionPickerLimit = 50

func promptSelectSessionID(
	ctx context.Context,
	svc *api.Service,
	req api.ListSessionsRequest,
	label string,
) (string, error) {
	if !stdinIsTTY() {
		return "", fmt.Errorf("session id required in non-interactive use")
	}
	req.Limit = sessionPickerLimit

	var items []api.SessionListItem
	if err := runWithProgress(ctx, "sessions", "loading sessions", func() error {
		var err error
		items, err = listSessionItemsWithProjectFallback(ctx, svc, req, req.Project != "")
		return err
	}); err != nil {
		return "", err
	}
	if len(items) == 0 {
		return "", fmt.Errorf("no sessions found")
	}

	filter, err := promptSessionFilter(ctx)
	if err != nil {
		return "", err
	}
	filtered := filterSessionItems(items, filter)
	if len(filtered) == 0 {
		return "", fmt.Errorf("no sessions match %q", filter)
	}
	return promptSessionChoice(ctx, filtered, label)
}

func promptSessionFilter(ctx context.Context) (string, error) {
	w, err := wizard.New(wizard.TextInput(
		"filter", "Filter sessions (blank for recent)",
	))
	if err != nil {
		return "", err
	}
	if err := wizard.RunLine(ctx, w, os.Stdin, os.Stderr); err != nil {
		return "", err
	}
	return strings.TrimSpace(wizard.String(w.Results(), "filter")), nil
}

func promptSessionChoice(
	ctx context.Context,
	items []api.SessionListItem,
	label string,
) (string, error) {
	options := make([]wizard.Option, 0, len(items))
	rows := toItemRows(items, "table")
	for i, item := range items {
		row := rows[i]
		options = append(options, wizard.Option{
			Value: item.Session.ID,
			Label: fmt.Sprintf(
				"%s  %s  %s  %s  %s",
				row.ID, row.Source, row.Project, row.Started, row.Actions,
			),
		})
	}
	w, err := wizard.New(wizard.Select("session", label, options).WithRequired())
	if err != nil {
		return "", err
	}
	if err := wizard.RunLine(ctx, w, os.Stdin, os.Stderr); err != nil {
		return "", err
	}
	return wizard.Choice(w.Results(), "session"), nil
}

func filterSessionItems(
	items []api.SessionListItem,
	filter string,
) []api.SessionListItem {
	filter = strings.ToLower(strings.TrimSpace(filter))
	if filter == "" {
		return items
	}
	var out []api.SessionListItem
	for _, item := range items {
		s := item.Session
		hay := strings.ToLower(strings.Join([]string{
			s.ID,
			s.NativeID,
			string(s.CLI),
			projectName(s.ProjectCwd),
			s.ProjectCwd,
			item.Actions,
		}, " "))
		if strings.Contains(hay, filter) {
			out = append(out, item)
		}
	}
	return out
}

func stdinIsTTY() bool {
	return isatty.IsTerminal(os.Stdin.Fd())
}
