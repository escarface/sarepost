package parity_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/escarface/sarepost/internal/application/generation"
	"github.com/escarface/sarepost/internal/capabilities"
	"github.com/escarface/sarepost/internal/db"
	"github.com/escarface/sarepost/internal/domain"
)

func contentPlanCapabilityChecks(action string) map[capabilities.Surface]parityCheck {
	checks := make(map[capabilities.Surface]parityCheck, 3)
	for _, candidate := range []capabilities.Surface{capabilities.SurfaceAPI, capabilities.SurfaceCLI, capabilities.SurfaceMCP} {
		surface := candidate
		checks[surface] = func(env *parityEnv) error { return checkContentPlanAction(env, surface, action) }
	}
	return checks
}

func checkContentPlanAction(env *parityEnv, surface capabilities.Surface, action string) error {
	profileID, err := ensureParityBrandProfile(env)
	if err != nil {
		return err
	}
	payload := parityContentPlanPayload(env, profileID)
	if action == "preview" || action == "create" {
		return invokeContentPlanBuild(env, surface, action, payload)
	}
	if action == "update" {
		planID, err := createParityContentPlanAPI(env, payload)
		if err != nil {
			return err
		}
		payload["name"] = "Updated parity plan"
		return invokeContentPlanUpdate(env, surface, planID, payload)
	}
	if action == "list" {
		switch surface {
		case capabilities.SurfaceAPI:
			_, status := env.apiJSON(http.MethodGet, "/content-plans", nil, "")
			if status != http.StatusOK {
				return fmt.Errorf("list status %d", status)
			}
		case capabilities.SurfaceCLI:
			code, _, stderr := env.runCLIResult("content-plans", "list")
			if code != 0 {
				return fmt.Errorf("list cli: %s", stderr)
			}
		case capabilities.SurfaceMCP:
			if msg := env.mcpCallToolError("postflow_list_content_plans", map[string]any{}); msg != "" {
				return fmt.Errorf("list mcp: %s", msg)
			}
		}
		return nil
	}

	var planID, variantID string
	if action == "edit" || action == "retry" || action == "regenerate" || action == "schedule" {
		status := domain.ContentPlanVariantReady
		if action == "retry" {
			status = domain.ContentPlanVariantFailed
		}
		planID, variantID, err = seedParityContentPlanVariant(env, status)
	} else {
		planID, err = createParityContentPlanAPI(env, payload)
	}
	if err != nil {
		return err
	}

	switch action {
	case "get":
		return invokePlanIDAction(env, surface, action, planID, "")
	case "generate", "cancel":
		return invokePlanIDAction(env, surface, action, planID, "")
	case "retry":
		return invokePlanIDAction(env, surface, action, planID, variantID)
	case "regenerate":
		return invokePlanIDAction(env, surface, action, planID, variantID)
	case "edit":
		return invokePlanIDAction(env, surface, action, planID, variantID)
	case "schedule":
		return invokePlanIDAction(env, surface, action, planID, variantID)
	default:
		return fmt.Errorf("unknown content plan parity action %s", action)
	}
}

func ensureParityBrandProfile(env *parityEnv) (string, error) {
	profile := generation.BrandProfile{ID: "brand_parity", Name: "Parity Brand", Tone: "clear"}
	raw, _ := json.Marshal([]generation.BrandProfile{profile})
	if err := env.store.SetSetting(env.t.Context(), generation.SettingBrandProfiles, string(raw)); err != nil {
		return "", err
	}
	return profile.ID, nil
}

func parityContentPlanPayload(env *parityEnv, profileID string) map[string]any {
	from := time.Now().UTC().Add(48 * time.Hour).Truncate(24 * time.Hour)
	to := from.Add(23*time.Hour + 59*time.Minute)
	return map[string]any{
		"name": fmt.Sprintf("Parity plan %d", env.nextReq), "objective": "Parity", "from": from.Format(time.RFC3339), "to": to.Format(time.RFC3339), "timezone": "UTC",
		"blocks": []map[string]any{{"brand_profile_id": profileID, "account_ids": []string{env.account.ID}, "weekdays": []string{strings.ToLower(from.Weekday().String())}, "slots": []string{"09:00"}}},
	}
}

func invokeContentPlanBuild(env *parityEnv, surface capabilities.Surface, action string, payload map[string]any) error {
	path := "/content-plans"
	tool := "postflow_create_content_plan"
	if action == "preview" {
		path += "/preview"
		tool = "postflow_preview_content_plan"
	}
	switch surface {
	case capabilities.SurfaceAPI:
		_, status := env.apiJSON(http.MethodPost, path, payload, "application/json")
		expected := http.StatusCreated
		if action == "preview" {
			expected = http.StatusOK
		}
		if status != expected {
			return fmt.Errorf("%s api status %d", action, status)
		}
	case capabilities.SurfaceMCP:
		if msg := env.mcpCallToolError(tool, payload); msg != "" {
			return fmt.Errorf("%s mcp: %s", action, msg)
		}
	case capabilities.SurfaceCLI:
		blocks, _ := json.Marshal(payload["blocks"])
		code, _, stderr := env.runCLIResult("content-plans", action, "--name", payload["name"].(string), "--from", payload["from"].(string), "--to", payload["to"].(string), "--timezone", "UTC", "--blocks-json", string(blocks))
		if code != 0 {
			return fmt.Errorf("%s cli: %s", action, stderr)
		}
	}
	return nil
}

func createParityContentPlanAPI(env *parityEnv, payload map[string]any) (string, error) {
	raw, status := env.apiJSON(http.MethodPost, "/content-plans", payload, "application/json")
	if status != http.StatusCreated {
		return "", fmt.Errorf("create prerequisite status %d: %s", status, raw)
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	return out.ID, nil
}

func invokeContentPlanUpdate(env *parityEnv, surface capabilities.Surface, planID string, payload map[string]any) error {
	switch surface {
	case capabilities.SurfaceAPI:
		_, status := env.apiJSON(http.MethodPatch, "/content-plans/"+planID, payload, "application/json")
		if status != http.StatusOK {
			return fmt.Errorf("update api status %d", status)
		}
	case capabilities.SurfaceMCP:
		payload["plan_id"] = planID
		if msg := env.mcpCallToolError("postflow_update_content_plan", payload); msg != "" {
			return fmt.Errorf("update mcp: %s", msg)
		}
	case capabilities.SurfaceCLI:
		blocks, _ := json.Marshal(payload["blocks"])
		code, _, stderr := env.runCLIResult("content-plans", "update", "--id", planID, "--name", payload["name"].(string), "--from", payload["from"].(string), "--to", payload["to"].(string), "--timezone", "UTC", "--blocks-json", string(blocks))
		if code != 0 {
			return fmt.Errorf("update cli: %s", stderr)
		}
	}
	return nil
}

func seedParityContentPlanVariant(env *parityEnv, status domain.ContentPlanVariantStatus) (string, string, error) {
	blockID, _ := db.NewID("plb")
	itemID, _ := db.NewID("pli")
	variantID, _ := db.NewID("plv")
	when := time.Now().UTC().Add(time.Duration(env.nextReq+500) * time.Minute).Round(time.Second)
	plan, err := env.store.CreateContentPlan(env.t.Context(), domain.ContentPlan{
		Name: "Parity editorial", StartsAt: when, EndsAt: when, Status: domain.ContentPlanStatusReview,
		Blocks: []domain.ContentPlanBlock{{ID: blockID, AccountIDs: []string{env.account.ID}}},
		Items:  []domain.ContentPlanItem{{ID: itemID, BlockID: blockID, PlannedAt: when, Variants: []domain.ContentPlanVariant{{ID: variantID, AccountID: env.account.ID, Platform: env.account.Platform, Text: "Parity ready copy " + variantID, Status: status, PlannedAt: when}}}},
	})
	return plan.ID, variantID, err
}

func invokePlanIDAction(env *parityEnv, surface capabilities.Surface, action, planID, variantID string) error {
	path := "/content-plans/" + planID
	tool := "postflow_" + action + "_content_plan"
	payload := map[string]any{"plan_id": planID}
	method := http.MethodPost
	cliArgs := []string{"content-plans", action, "--id", planID}
	if action == "get" {
		method = http.MethodGet
		tool = "postflow_get_content_plan"
	}
	if action == "retry" || action == "regenerate" || action == "schedule" {
		payload["variant_ids"] = []string{variantID}
		path += "/" + action
		cliArgs = append(cliArgs, "--variant-id", variantID)
	}
	if action == "generate" || action == "cancel" {
		path += "/" + action
	}
	if action == "edit" {
		when := time.Now().UTC().Add(7 * 24 * time.Hour).Round(time.Second).Format(time.RFC3339)
		path += "/variants/" + variantID
		method = http.MethodPatch
		tool = "postflow_update_content_plan_variant"
		payload["variant_id"], payload["text"], payload["planned_at"] = variantID, "Edited parity copy", when
		cliArgs = append(cliArgs, "--variant-id", variantID, "--text", "Edited parity copy", "--planned-at", when)
	}
	switch surface {
	case capabilities.SurfaceAPI:
		body := any(payload)
		if method == http.MethodGet {
			body = nil
		}
		_, status := env.apiJSON(method, path, body, "application/json")
		if status < 200 || status >= 300 {
			return fmt.Errorf("%s api status %d", action, status)
		}
	case capabilities.SurfaceMCP:
		if msg := env.mcpCallToolError(tool, payload); msg != "" {
			return fmt.Errorf("%s mcp: %s", action, msg)
		}
	case capabilities.SurfaceCLI:
		code, _, stderr := env.runCLIResult(cliArgs...)
		if code != 0 {
			return fmt.Errorf("%s cli: %s", action, stderr)
		}
	}
	return nil
}
