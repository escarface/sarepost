package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/url"
	"strings"
)

type campaignsListResponse struct {
	Count int           `json:"count"`
	Items []campaignDTO `json:"items"`
}

type campaignDTO struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Objective      string   `json:"objective"`
	Status         string   `json:"status"`
	StartsAt       string   `json:"starts_at"`
	EndsAt         string   `json:"ends_at"`
	Tags           []string `json:"tags"`
	Timezone       string   `json:"timezone"`
	BrandProfileID string   `json:"brand_profile_id"`
}

type backlogResponse struct {
	Count int              `json:"count"`
	Items []backlogItemDTO `json:"items"`
}

type backlogItemDTO struct {
	Post     postDTO     `json:"post"`
	Campaign campaignDTO `json:"campaign"`
}

func runCampaigns(ctx context.Context, client *APIClient, cfg config, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: postflow campaigns <create|list|update|archive|add-post|create-drafts|backlog> [flags]")
		return 2
	}
	switch args[0] {
	case "create":
		return runCampaignCreate(ctx, client, cfg, args[1:], stdout, stderr)
	case "list":
		return runCampaignList(ctx, client, cfg, args[1:], stdout, stderr)
	case "update":
		return runCampaignUpdate(ctx, client, cfg, args[1:], stdout, stderr)
	case "archive":
		return runCampaignArchive(ctx, client, cfg, args[1:], stdout, stderr)
	case "add-post":
		return runCampaignAddPost(ctx, client, cfg, args[1:], stdout, stderr)
	case "create-drafts":
		return runCampaignCreateDrafts(ctx, client, cfg, args[1:], stdout, stderr)
	case "backlog":
		return runCampaignBacklog(ctx, client, cfg, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown campaigns subcommand: %s\n", args[0])
		return 2
	}
}

func runCampaignCreate(ctx context.Context, client *APIClient, cfg config, args []string, stdout, stderr io.Writer) int {
	fs, flags := campaignFlagSet("campaigns create", stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	payload := flags.payload()
	if strings.TrimSpace(payload["name"].(string)) == "" {
		fmt.Fprintln(stderr, "--name is required")
		return 2
	}
	var out campaignDTO
	if err := client.Post(ctx, "/campaigns", payload, &out); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	printOutput(stdout, cfg.asJSON, out, func() {
		fmt.Fprintf(stdout, "campaign created: %s %s\n", out.ID, out.Name)
	})
	return 0
}

func runCampaignList(ctx context.Context, client *APIClient, cfg config, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("campaigns list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	status := fs.String("status", "", "Optional status: active|paused|archived")
	tag := fs.String("tag", "", "Optional tag filter")
	limit := fs.Int("limit", 200, "Max campaigns")
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
	if *limit > 0 {
		query.Set("limit", fmt.Sprintf("%d", *limit))
	}
	var out campaignsListResponse
	if err := client.Get(ctx, "/campaigns", query, &out); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	printOutput(stdout, cfg.asJSON, out, func() {
		fmt.Fprintf(stdout, "count: %d\n", out.Count)
		for _, item := range out.Items {
			fmt.Fprintf(stdout, "- [%s] %s · %s\n", item.Status, item.ID, item.Name)
		}
	})
	return 0
}

func runCampaignUpdate(ctx context.Context, client *APIClient, cfg config, args []string, stdout, stderr io.Writer) int {
	fs, flags := campaignFlagSet("campaigns update", stderr)
	id := fs.String("id", "", "Campaign ID")
	status := fs.String("status", "", "Optional status: active|paused|archived")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	payload := flags.payload()
	if strings.TrimSpace(*id) == "" {
		fmt.Fprintln(stderr, "--id is required")
		return 2
	}
	if strings.TrimSpace(*status) != "" {
		payload["status"] = strings.TrimSpace(*status)
	}
	var out campaignDTO
	if err := client.Post(ctx, "/campaigns/"+strings.TrimSpace(*id), payload, &out); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	printOutput(stdout, cfg.asJSON, out, func() {
		fmt.Fprintf(stdout, "campaign updated: %s %s\n", out.ID, out.Name)
	})
	return 0
}

func runCampaignArchive(ctx context.Context, client *APIClient, cfg config, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("campaigns archive", flag.ContinueOnError)
	fs.SetOutput(stderr)
	id := fs.String("id", "", "Campaign ID")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*id) == "" {
		fmt.Fprintln(stderr, "--id is required")
		return 2
	}
	var out campaignDTO
	if err := client.Post(ctx, "/campaigns/"+strings.TrimSpace(*id)+"/archive", map[string]any{}, &out); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	printOutput(stdout, cfg.asJSON, out, func() {
		fmt.Fprintf(stdout, "campaign archived: %s\n", out.ID)
	})
	return 0
}

func runCampaignAddPost(ctx context.Context, client *APIClient, cfg config, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("campaigns add-post", flag.ContinueOnError)
	fs.SetOutput(stderr)
	id := fs.String("id", "", "Campaign ID")
	postID := fs.String("post-id", "", "Post ID")
	editorialStatus := fs.String("editorial-status", "drafting", "Editorial status")
	requiresApproval := fs.Bool("requires-approval", false, "Require approval before scheduling")
	tags := fs.String("tags", "", "Comma-separated tags")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*id) == "" || strings.TrimSpace(*postID) == "" {
		fmt.Fprintln(stderr, "--id and --post-id are required")
		return 2
	}
	payload := map[string]any{
		"post_id":           strings.TrimSpace(*postID),
		"editorial_status":  strings.TrimSpace(*editorialStatus),
		"requires_approval": *requiresApproval,
		"tags":              splitCLIList(*tags),
	}
	var out map[string]any
	if err := client.Post(ctx, "/campaigns/"+strings.TrimSpace(*id)+"/posts", payload, &out); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	printOutput(stdout, cfg.asJSON, out, func() {
		fmt.Fprintf(stdout, "post added to campaign: %s\n", strings.TrimSpace(*postID))
	})
	return 0
}

func runCampaignCreateDrafts(ctx context.Context, client *APIClient, cfg config, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("campaigns create-drafts", flag.ContinueOnError)
	fs.SetOutput(stderr)
	id := fs.String("id", "", "Campaign ID")
	accountID := fs.String("account-id", "", "Primary account ID")
	accountIDs := fs.String("account-ids", "", "Comma-separated account IDs")
	idea := fs.String("idea", "", "Campaign idea or angle")
	variants := fs.Int("variants-per-post", 1, "Variants per account")
	brandProfileID := fs.String("brand-profile-id", "", "Optional brand profile ID")
	brandProfileName := fs.String("brand-profile", "", "Optional brand profile name")
	editorialStatus := fs.String("editorial-status", "needs_review", "Editorial status")
	requiresApproval := fs.Bool("requires-approval", true, "Require approval before scheduling")
	tags := fs.String("tags", "", "Comma-separated tags")
	idempotencyPrefix := fs.String("idempotency-prefix", "", "Optional idempotency prefix")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*id) == "" {
		fmt.Fprintln(stderr, "--id is required")
		return 2
	}
	ids := splitCLIList(*accountIDs)
	if strings.TrimSpace(*accountID) != "" {
		ids = append(ids, strings.TrimSpace(*accountID))
	}
	if len(ids) == 0 {
		fmt.Fprintln(stderr, "--account-id or --account-ids is required")
		return 2
	}
	payload := map[string]any{
		"account_ids":        ids,
		"idea":               strings.TrimSpace(*idea),
		"variants_per_post":  *variants,
		"brand_profile_id":   strings.TrimSpace(*brandProfileID),
		"brand_profile":      strings.TrimSpace(*brandProfileName),
		"editorial_status":   strings.TrimSpace(*editorialStatus),
		"requires_approval":  *requiresApproval,
		"tags":               splitCLIList(*tags),
		"idempotency_prefix": strings.TrimSpace(*idempotencyPrefix),
	}
	var out map[string]any
	if err := client.Post(ctx, "/campaigns/"+strings.TrimSpace(*id)+"/drafts", payload, &out); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	printOutput(stdout, cfg.asJSON, out, func() {
		fmt.Fprintf(stdout, "campaign drafts created\n")
	})
	return 0
}

func runCampaignBacklog(ctx context.Context, client *APIClient, cfg config, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("campaigns backlog", flag.ContinueOnError)
	fs.SetOutput(stderr)
	campaignID := fs.String("campaign-id", "", "Optional campaign ID")
	platform := fs.String("platform", "", "Optional platform")
	editorialStatus := fs.String("editorial-status", "", "Optional editorial status")
	tag := fs.String("tag", "", "Optional tag")
	limit := fs.Int("limit", 200, "Max items")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	query := url.Values{}
	if strings.TrimSpace(*campaignID) != "" {
		query.Set("campaign_id", strings.TrimSpace(*campaignID))
	}
	if strings.TrimSpace(*platform) != "" {
		query.Set("platform", strings.TrimSpace(*platform))
	}
	if strings.TrimSpace(*editorialStatus) != "" {
		query.Set("editorial_status", strings.TrimSpace(*editorialStatus))
	}
	if strings.TrimSpace(*tag) != "" {
		query.Set("tag", strings.TrimSpace(*tag))
	}
	if *limit > 0 {
		query.Set("limit", fmt.Sprintf("%d", *limit))
	}
	var out backlogResponse
	if err := client.Get(ctx, "/editorial/backlog", query, &out); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	printOutput(stdout, cfg.asJSON, out, func() {
		fmt.Fprintf(stdout, "count: %d\n", out.Count)
		for _, item := range out.Items {
			fmt.Fprintf(stdout, "- [%s] %s · %s\n", item.Post.Status, item.Post.ID, oneLine(item.Post.Text, 90))
		}
	})
	return 0
}

func runPostsApprove(ctx context.Context, client *APIClient, cfg config, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("posts approve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	id := fs.String("id", "", "Post ID")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*id) == "" {
		fmt.Fprintln(stderr, "--id is required")
		return 2
	}
	var out map[string]any
	if err := client.Post(ctx, "/posts/"+strings.TrimSpace(*id)+"/approve", map[string]any{}, &out); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	printOutput(stdout, cfg.asJSON, out, func() {
		fmt.Fprintf(stdout, "post approved: %s\n", strings.TrimSpace(*id))
	})
	return 0
}

type campaignFlags struct {
	name             *string
	objective        *string
	startsAt         *string
	endsAt           *string
	notes            *string
	tags             *string
	timezone         *string
	audience         *string
	tone             *string
	cta              *string
	restrictions     *string
	brandProfileID   *string
	brandProfileName *string
}

func campaignFlagSet(name string, stderr io.Writer) (*flag.FlagSet, campaignFlags) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	flags := campaignFlags{
		name:             fs.String("name", "", "Campaign name"),
		objective:        fs.String("objective", "", "Campaign objective"),
		startsAt:         fs.String("starts-at", "", "Start date RFC3339"),
		endsAt:           fs.String("ends-at", "", "End date RFC3339"),
		notes:            fs.String("notes", "", "Notes"),
		tags:             fs.String("tags", "", "Comma-separated tags"),
		timezone:         fs.String("timezone", "", "Campaign timezone"),
		audience:         fs.String("audience", "", "Audience brief"),
		tone:             fs.String("tone", "", "Tone brief"),
		cta:              fs.String("cta", "", "Call to action"),
		restrictions:     fs.String("restrictions", "", "Restrictions"),
		brandProfileID:   fs.String("brand-profile-id", "", "Brand profile ID"),
		brandProfileName: fs.String("brand-profile", "", "Brand profile name"),
	}
	return fs, flags
}

func (f campaignFlags) payload() map[string]any {
	return map[string]any{
		"name":             strings.TrimSpace(*f.name),
		"objective":        strings.TrimSpace(*f.objective),
		"starts_at":        strings.TrimSpace(*f.startsAt),
		"ends_at":          strings.TrimSpace(*f.endsAt),
		"notes":            strings.TrimSpace(*f.notes),
		"tags":             splitCLIList(*f.tags),
		"timezone":         strings.TrimSpace(*f.timezone),
		"audience":         strings.TrimSpace(*f.audience),
		"tone":             strings.TrimSpace(*f.tone),
		"cta":              strings.TrimSpace(*f.cta),
		"restrictions":     strings.TrimSpace(*f.restrictions),
		"brand_profile_id": strings.TrimSpace(*f.brandProfileID),
		"brand_profile":    strings.TrimSpace(*f.brandProfileName),
	}
}

func splitCLIList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			out = append(out, value)
		}
	}
	return out
}
