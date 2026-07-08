package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/url"
	"strings"
)

func runContentSources(ctx context.Context, client *APIClient, cfg config, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: postflow content-sources <create|list|get|update|archive|generate-angles> [flags]")
		return 2
	}
	switch args[0] {
	case "create", "update":
		return runContentSourceWrite(ctx, client, cfg, args[0], args[1:], stdout, stderr)
	case "list":
		return runContentSourceList(ctx, client, cfg, args[1:], stdout, stderr)
	case "get", "archive":
		return runContentSourceIDAction(ctx, client, cfg, args[0], args[1:], stdout, stderr)
	case "generate-angles":
		return runContentSourceGenerateAngles(ctx, client, cfg, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown content-sources subcommand: %s\n", args[0])
		return 2
	}
}

func runContentSourceWrite(ctx context.Context, client *APIClient, cfg config, action string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("content-sources "+action, flag.ContinueOnError)
	fs.SetOutput(stderr)
	id := fs.String("id", "", "Content source ID (required for update)")
	title := fs.String("title", "", "Content source title")
	body := fs.String("body", "", "Raw source body")
	sourceURL := fs.String("source-url", "", "Optional reference URL")
	campaignID := fs.String("campaign-id", "", "Optional campaign ID")
	brandProfileID := fs.String("brand-profile-id", "", "Optional brand profile ID")
	status := fs.String("status", "", "Optional status for update: new|processed|archived")
	var tags stringListFlag
	fs.Var(&tags, "tag", "Tag; repeat for multiple")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if action == "create" && (strings.TrimSpace(*title) == "" || strings.TrimSpace(*body) == "") {
		fmt.Fprintln(stderr, "--title and --body are required")
		return 2
	}
	payload := map[string]any{
		"title":            *title,
		"body":             *body,
		"source_url":       *sourceURL,
		"campaign_id":      *campaignID,
		"brand_profile_id": *brandProfileID,
		"tags":             []string(tags),
	}
	var out map[string]any
	var err error
	if action == "update" {
		if strings.TrimSpace(*id) == "" {
			fmt.Fprintln(stderr, "--id is required for update")
			return 2
		}
		if strings.TrimSpace(*status) != "" {
			payload["status"] = *status
		}
		err = client.Patch(ctx, "/content-sources/"+url.PathEscape(strings.TrimSpace(*id)), payload, &out)
	} else {
		err = client.Post(ctx, "/content-sources", payload, &out)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	printOutput(stdout, cfg.asJSON, out, func() { fmt.Fprintf(stdout, "content source %s complete\n", action) })
	return 0
}

func runContentSourceList(ctx context.Context, client *APIClient, cfg config, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("content-sources list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	status := fs.String("status", "", "Optional status filter")
	tag := fs.String("tag", "", "Optional tag filter")
	limit := fs.String("limit", "", "Optional result limit")
	includeArchived := fs.Bool("include-archived", false, "Include archived sources")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	query := url.Values{}
	if strings.TrimSpace(*status) != "" {
		query.Set("status", strings.TrimSpace(*status))
	}
	if strings.TrimSpace(*tag) != "" {
		query.Set("tag", strings.TrimSpace(*tag))
	}
	if strings.TrimSpace(*limit) != "" {
		query.Set("limit", strings.TrimSpace(*limit))
	}
	if *includeArchived {
		query.Set("include_archived", "true")
	}
	var out map[string]any
	if err := client.Get(ctx, "/content-sources", query, &out); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	printOutput(stdout, cfg.asJSON, out, func() { fmt.Fprintln(stdout, "content sources listed") })
	return 0
}

func runContentSourceIDAction(ctx context.Context, client *APIClient, cfg config, action string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("content-sources "+action, flag.ContinueOnError)
	fs.SetOutput(stderr)
	id := fs.String("id", "", "Content source ID")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*id) == "" {
		fmt.Fprintln(stderr, "--id is required")
		return 2
	}
	path := "/content-sources/" + url.PathEscape(strings.TrimSpace(*id))
	var out map[string]any
	var err error
	if action == "get" {
		err = client.Get(ctx, path, nil, &out)
	} else {
		err = client.Post(ctx, path+"/"+action, map[string]any{}, &out)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	printOutput(stdout, cfg.asJSON, out, func() { fmt.Fprintf(stdout, "content source %s complete\n", action) })
	return 0
}

func runContentSourceGenerateAngles(ctx context.Context, client *APIClient, cfg config, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("content-sources generate-angles", flag.ContinueOnError)
	fs.SetOutput(stderr)
	id := fs.String("id", "", "Content source ID")
	count := fs.Int("count", 5, "Number of angles")
	instructions := fs.String("instructions", "", "Optional generation instructions")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*id) == "" {
		fmt.Fprintln(stderr, "--id is required")
		return 2
	}
	var out map[string]any
	if err := client.Post(ctx, "/content-sources/"+url.PathEscape(strings.TrimSpace(*id))+"/generate-angles", map[string]any{"count": *count, "instructions": *instructions}, &out); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	printOutput(stdout, cfg.asJSON, out, func() { fmt.Fprintln(stdout, "content source angles generated") })
	return 0
}
