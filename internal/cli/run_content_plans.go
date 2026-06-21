package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/url"
	"strings"
)

func runContentPlans(ctx context.Context, client *APIClient, cfg config, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: postflow content-plans <preview|create|update|list|get|generate|edit|retry|regenerate|schedule|cancel> [flags]")
		return 2
	}
	switch args[0] {
	case "preview", "create", "update":
		return runContentPlanBuild(ctx, client, cfg, args[0], args[1:], stdout, stderr)
	case "list":
		return runContentPlanList(ctx, client, cfg, args[1:], stdout, stderr)
	case "get", "generate", "cancel":
		return runContentPlanIDAction(ctx, client, cfg, args[0], args[1:], stdout, stderr)
	case "retry", "regenerate", "schedule":
		return runContentPlanVariantsAction(ctx, client, cfg, args[0], args[1:], stdout, stderr)
	case "edit":
		return runContentPlanEdit(ctx, client, cfg, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown content-plans subcommand: %s\n", args[0])
		return 2
	}
}

func runContentPlanBuild(ctx context.Context, client *APIClient, cfg config, action string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("content-plans "+action, flag.ContinueOnError)
	fs.SetOutput(stderr)
	name := fs.String("name", "", "Plan name")
	objective := fs.String("objective", "", "Plan objective")
	from := fs.String("from", "", "RFC3339 range start")
	to := fs.String("to", "", "RFC3339 range end")
	timezone := fs.String("timezone", "UTC", "IANA timezone")
	blocksJSON := fs.String("blocks-json", "", "JSON array of brand/account cadence blocks")
	id := fs.String("id", "", "Content plan ID (required for update)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*name) == "" || strings.TrimSpace(*from) == "" || strings.TrimSpace(*to) == "" || strings.TrimSpace(*blocksJSON) == "" {
		fmt.Fprintln(stderr, "--name, --from, --to and --blocks-json are required")
		return 2
	}
	var blocks []map[string]any
	if err := json.Unmarshal([]byte(*blocksJSON), &blocks); err != nil {
		fmt.Fprintf(stderr, "invalid --blocks-json: %v\n", err)
		return 2
	}
	payload := map[string]any{"name": *name, "objective": *objective, "from": *from, "to": *to, "timezone": *timezone, "blocks": blocks}
	path := "/content-plans"
	if action == "preview" {
		path += "/preview"
	}
	var out map[string]any
	var requestErr error
	if action == "update" {
		if strings.TrimSpace(*id) == "" {
			fmt.Fprintln(stderr, "--id is required for update")
			return 2
		}
		requestErr = client.Patch(ctx, path+"/"+url.PathEscape(strings.TrimSpace(*id)), payload, &out)
	} else {
		requestErr = client.Post(ctx, path, payload, &out)
	}
	if requestErr != nil {
		fmt.Fprintln(stderr, requestErr)
		return 1
	}
	printOutput(stdout, cfg.asJSON, out, func() { fmt.Fprintf(stdout, "content plan %s complete\n", action) })
	return 0
}

func runContentPlanList(ctx context.Context, client *APIClient, cfg config, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("content-plans list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	var out map[string]any
	if err := client.Get(ctx, "/content-plans", url.Values{}, &out); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	printOutput(stdout, cfg.asJSON, out, func() { fmt.Fprintln(stdout, "content plans listed") })
	return 0
}

func runContentPlanIDAction(ctx context.Context, client *APIClient, cfg config, action string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("content-plans "+action, flag.ContinueOnError)
	fs.SetOutput(stderr)
	id := fs.String("id", "", "Content plan ID")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*id) == "" {
		fmt.Fprintln(stderr, "--id is required")
		return 2
	}
	path := "/content-plans/" + url.PathEscape(strings.TrimSpace(*id))
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
	printOutput(stdout, cfg.asJSON, out, func() { fmt.Fprintf(stdout, "content plan %s complete\n", action) })
	return 0
}

func runContentPlanVariantsAction(ctx context.Context, client *APIClient, cfg config, action string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("content-plans "+action, flag.ContinueOnError)
	fs.SetOutput(stderr)
	id := fs.String("id", "", "Content plan ID")
	var variantIDs stringListFlag
	var itemIDs stringListFlag
	fs.Var(&variantIDs, "variant-id", "Variant ID; repeat for multiple")
	fs.Var(&itemIDs, "item-id", "Editorial item ID; repeat for multiple (regenerate only)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*id) == "" || (len(variantIDs) == 0 && (action != "regenerate" || len(itemIDs) == 0)) {
		fmt.Fprintln(stderr, "--id and at least one --variant-id or --item-id are required")
		return 2
	}
	endpointAction := action
	var out map[string]any
	if err := client.Post(ctx, "/content-plans/"+url.PathEscape(*id)+"/"+endpointAction, map[string]any{"variant_ids": []string(variantIDs), "item_ids": []string(itemIDs)}, &out); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	printOutput(stdout, cfg.asJSON, out, func() { fmt.Fprintf(stdout, "content plan %s complete\n", action) })
	return 0
}

func runContentPlanEdit(ctx context.Context, client *APIClient, cfg config, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("content-plans edit", flag.ContinueOnError)
	fs.SetOutput(stderr)
	id := fs.String("id", "", "Content plan ID")
	variantID := fs.String("variant-id", "", "Variant ID")
	text := fs.String("text", "", "Final post copy")
	plannedAt := fs.String("planned-at", "", "RFC3339 planned time")
	mediaID := fs.String("media-id", "", "Optional media ID")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *id == "" || *variantID == "" || *text == "" || *plannedAt == "" {
		fmt.Fprintln(stderr, "--id, --variant-id, --text and --planned-at are required")
		return 2
	}
	var out map[string]any
	path := "/content-plans/" + url.PathEscape(*id) + "/variants/" + url.PathEscape(*variantID)
	if err := client.Patch(ctx, path, map[string]any{"text": *text, "planned_at": *plannedAt, "media_id": *mediaID}, &out); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	printOutput(stdout, cfg.asJSON, out, func() { fmt.Fprintln(stdout, "content plan variant updated") })
	return 0
}
