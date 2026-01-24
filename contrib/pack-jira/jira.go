// Package jira provides Jira integration tools for agent-go.
//
// Tools include issue management, project tracking, sprints, and search.
package jira

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/andygrunwald/go-jira"
	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// Config holds Jira connection settings.
type Config struct {
	BaseURL  string
	Username string
	APIToken string
}

// Pack returns the Jira tool pack.
func Pack(cfg Config) *pack.Pack {
	p := &jiraPack{cfg: cfg}

	return pack.NewBuilder("jira").
		WithDescription("Jira integration tools for issue tracking, projects, and sprints").
		WithVersion("1.0.0").
		AddTools(
			p.getIssueTool(),
			p.createIssueTool(),
			p.updateIssueTool(),
			p.transitionIssueTool(),
			p.addCommentTool(),
			p.getCommentsTool(),
			p.assignIssueTool(),
			p.linkIssuesTool(),
			p.getProjectTool(),
			p.listProjectsTool(),
			p.searchIssuesTool(),
			p.getSprintTool(),
			p.listSprintsTool(),
			p.getSprintIssuesTool(),
			p.moveToSprintTool(),
			p.getUserTool(),
			p.searchUsersTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

type jiraPack struct {
	cfg    Config
	client *jira.Client
}

func (p *jiraPack) getClient() (*jira.Client, error) {
	if p.client != nil {
		return p.client, nil
	}

	tp := jira.BasicAuthTransport{
		Username: p.cfg.Username,
		Password: p.cfg.APIToken,
	}

	client, err := jira.NewClient(tp.Client(), p.cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create Jira client: %w", err)
	}

	p.client = client
	return p.client, nil
}

// ============================================================================
// Issue Tools
// ============================================================================

func (p *jiraPack) getIssueTool() tool.Tool {
	return tool.NewBuilder("jira_get_issue").
		WithDescription("Get a Jira issue by key").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Key    string   `json:"key"`
				Fields []string `json:"fields,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			client, err := p.getClient()
			if err != nil {
				return tool.Result{}, err
			}

			opts := &jira.GetQueryOptions{}
			if len(in.Fields) > 0 {
				opts.Fields = in.Fields[0]
				for i := 1; i < len(in.Fields); i++ {
					opts.Fields += "," + in.Fields[i]
				}
			}

			issue, _, err := client.Issue.Get(in.Key, opts)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get issue: %w", err)
			}

			output, _ := json.Marshal(formatIssue(issue))
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *jiraPack) createIssueTool() tool.Tool {
	return tool.NewBuilder("jira_create_issue").
		WithDescription("Create a new Jira issue").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Project     string            `json:"project"`
				IssueType   string            `json:"issue_type"`
				Summary     string            `json:"summary"`
				Description string            `json:"description,omitempty"`
				Priority    string            `json:"priority,omitempty"`
				Labels      []string          `json:"labels,omitempty"`
				Components  []string          `json:"components,omitempty"`
				Assignee    string            `json:"assignee,omitempty"`
				CustomFields map[string]any   `json:"custom_fields,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			client, err := p.getClient()
			if err != nil {
				return tool.Result{}, err
			}

			issue := &jira.Issue{
				Fields: &jira.IssueFields{
					Project:     jira.Project{Key: in.Project},
					Type:        jira.IssueType{Name: in.IssueType},
					Summary:     in.Summary,
					Description: in.Description,
				},
			}

			if in.Priority != "" {
				issue.Fields.Priority = &jira.Priority{Name: in.Priority}
			}
			if len(in.Labels) > 0 {
				issue.Fields.Labels = in.Labels
			}
			if len(in.Components) > 0 {
				components := make([]*jira.Component, len(in.Components))
				for i, c := range in.Components {
					components[i] = &jira.Component{Name: c}
				}
				issue.Fields.Components = components
			}
			if in.Assignee != "" {
				issue.Fields.Assignee = &jira.User{AccountID: in.Assignee}
			}

			created, _, err := client.Issue.Create(issue)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to create issue: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"key":  created.Key,
				"id":   created.ID,
				"self": created.Self,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *jiraPack) updateIssueTool() tool.Tool {
	return tool.NewBuilder("jira_update_issue").
		WithDescription("Update a Jira issue").
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Key         string   `json:"key"`
				Summary     string   `json:"summary,omitempty"`
				Description string   `json:"description,omitempty"`
				Priority    string   `json:"priority,omitempty"`
				Labels      []string `json:"labels,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			client, err := p.getClient()
			if err != nil {
				return tool.Result{}, err
			}

			update := make(map[string]interface{})
			fields := make(map[string]interface{})

			if in.Summary != "" {
				fields["summary"] = in.Summary
			}
			if in.Description != "" {
				fields["description"] = in.Description
			}
			if in.Priority != "" {
				fields["priority"] = map[string]string{"name": in.Priority}
			}
			if len(in.Labels) > 0 {
				fields["labels"] = in.Labels
			}

			if len(fields) > 0 {
				update["fields"] = fields
			}

			_, err = client.Issue.UpdateIssue(in.Key, update)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to update issue: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"key":     in.Key,
				"success": true,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *jiraPack) transitionIssueTool() tool.Tool {
	return tool.NewBuilder("jira_transition_issue").
		WithDescription("Transition a Jira issue to a new status").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Key        string `json:"key"`
				Transition string `json:"transition"` // transition name or ID
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			client, err := p.getClient()
			if err != nil {
				return tool.Result{}, err
			}

			// Get available transitions
			transitions, _, err := client.Issue.GetTransitions(in.Key)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get transitions: %w", err)
			}

			// Find matching transition
			var transitionID string
			for _, t := range transitions {
				if t.Name == in.Transition || t.ID == in.Transition {
					transitionID = t.ID
					break
				}
			}

			if transitionID == "" {
				available := make([]string, len(transitions))
				for i, t := range transitions {
					available[i] = t.Name
				}
				return tool.Result{}, fmt.Errorf("transition not found, available: %v", available)
			}

			_, err = client.Issue.DoTransition(in.Key, transitionID)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to transition: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"key":        in.Key,
				"transition": in.Transition,
				"success":    true,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *jiraPack) addCommentTool() tool.Tool {
	return tool.NewBuilder("jira_add_comment").
		WithDescription("Add a comment to a Jira issue").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Key  string `json:"key"`
				Body string `json:"body"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			client, err := p.getClient()
			if err != nil {
				return tool.Result{}, err
			}

			comment, _, err := client.Issue.AddComment(in.Key, &jira.Comment{
				Body: in.Body,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to add comment: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"id":      comment.ID,
				"created": comment.Created,
				"author":  comment.Author.DisplayName,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *jiraPack) getCommentsTool() tool.Tool {
	return tool.NewBuilder("jira_get_comments").
		WithDescription("Get comments on a Jira issue").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Key string `json:"key"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			client, err := p.getClient()
			if err != nil {
				return tool.Result{}, err
			}

			issue, _, err := client.Issue.Get(in.Key, &jira.GetQueryOptions{
				Fields: "comment",
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get comments: %w", err)
			}

			comments := make([]map[string]any, 0)
			if issue.Fields.Comments != nil {
				for _, c := range issue.Fields.Comments.Comments {
					comments = append(comments, map[string]any{
						"id":      c.ID,
						"body":    c.Body,
						"author":  c.Author.DisplayName,
						"created": c.Created,
						"updated": c.Updated,
					})
				}
			}

			output, _ := json.Marshal(map[string]any{
				"count":    len(comments),
				"comments": comments,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *jiraPack) assignIssueTool() tool.Tool {
	return tool.NewBuilder("jira_assign_issue").
		WithDescription("Assign a Jira issue to a user").
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Key      string `json:"key"`
				Assignee string `json:"assignee"` // account ID or empty to unassign
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			client, err := p.getClient()
			if err != nil {
				return tool.Result{}, err
			}

			var user *jira.User
			if in.Assignee != "" {
				user = &jira.User{AccountID: in.Assignee}
			}

			_, err = client.Issue.UpdateAssignee(in.Key, user)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to assign: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"key":      in.Key,
				"assignee": in.Assignee,
				"success":  true,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *jiraPack) linkIssuesTool() tool.Tool {
	return tool.NewBuilder("jira_link_issues").
		WithDescription("Create a link between two Jira issues").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				InwardKey  string `json:"inward_key"`
				OutwardKey string `json:"outward_key"`
				LinkType   string `json:"link_type"` // e.g., "Blocks", "Relates"
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			client, err := p.getClient()
			if err != nil {
				return tool.Result{}, err
			}

			_, err = client.Issue.AddLink(&jira.IssueLink{
				Type: jira.IssueLinkType{Name: in.LinkType},
				InwardIssue: &jira.Issue{
					Key: in.InwardKey,
				},
				OutwardIssue: &jira.Issue{
					Key: in.OutwardKey,
				},
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to link issues: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"success": true,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// ============================================================================
// Project Tools
// ============================================================================

func (p *jiraPack) getProjectTool() tool.Tool {
	return tool.NewBuilder("jira_get_project").
		WithDescription("Get a Jira project by key").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Key string `json:"key"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			client, err := p.getClient()
			if err != nil {
				return tool.Result{}, err
			}

			project, _, err := client.Project.Get(in.Key)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get project: %w", err)
			}

			issueTypes := make([]string, len(project.IssueTypes))
			for i, it := range project.IssueTypes {
				issueTypes[i] = it.Name
			}

			output, _ := json.Marshal(map[string]any{
				"key":         project.Key,
				"name":        project.Name,
				"description": project.Description,
				"lead":        project.Lead.DisplayName,
				"issue_types": issueTypes,
				"self":        project.Self,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *jiraPack) listProjectsTool() tool.Tool {
	return tool.NewBuilder("jira_list_projects").
		WithDescription("List Jira projects").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			client, err := p.getClient()
			if err != nil {
				return tool.Result{}, err
			}

			projects, _, err := client.Project.GetList()
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to list projects: %w", err)
			}

			result := make([]map[string]any, len(*projects))
			for i, proj := range *projects {
				result[i] = map[string]any{
					"key":  proj.Key,
					"name": proj.Name,
					"id":   proj.ID,
				}
			}

			output, _ := json.Marshal(map[string]any{
				"count":    len(result),
				"projects": result,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// ============================================================================
// Search Tools
// ============================================================================

func (p *jiraPack) searchIssuesTool() tool.Tool {
	return tool.NewBuilder("jira_search_issues").
		WithDescription("Search for Jira issues using JQL").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				JQL        string   `json:"jql"`
				MaxResults int      `json:"max_results,omitempty"`
				Fields     []string `json:"fields,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.MaxResults == 0 {
				in.MaxResults = 50
			}

			client, err := p.getClient()
			if err != nil {
				return tool.Result{}, err
			}

			opts := &jira.SearchOptions{
				MaxResults: in.MaxResults,
			}
			if len(in.Fields) > 0 {
				opts.Fields = in.Fields
			}

			issues, _, err := client.Issue.Search(in.JQL, opts)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to search: %w", err)
			}

			result := make([]map[string]any, len(issues))
			for i, issue := range issues {
				result[i] = formatIssue(&issue)
			}

			output, _ := json.Marshal(map[string]any{
				"count":  len(result),
				"issues": result,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// ============================================================================
// Sprint Tools
// ============================================================================

func (p *jiraPack) getSprintTool() tool.Tool {
	return tool.NewBuilder("jira_get_sprint").
		WithDescription("Get sprint issues by sprint name using JQL").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				SprintName string `json:"sprint_name"`
				MaxResults int    `json:"max_results,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.MaxResults == 0 {
				in.MaxResults = 50
			}

			client, err := p.getClient()
			if err != nil {
				return tool.Result{}, err
			}

			// Use JQL to find issues in the sprint
			jql := fmt.Sprintf("sprint = \"%s\"", in.SprintName)
			issues, _, err := client.Issue.Search(jql, &jira.SearchOptions{
				MaxResults: in.MaxResults,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get sprint issues: %w", err)
			}

			result := make([]map[string]any, len(issues))
			for i, issue := range issues {
				result[i] = formatIssue(&issue)
			}

			output, _ := json.Marshal(map[string]any{
				"sprint": in.SprintName,
				"count":  len(result),
				"issues": result,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *jiraPack) listSprintsTool() tool.Tool {
	return tool.NewBuilder("jira_list_sprints").
		WithDescription("List sprints for a board").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				BoardID int `json:"board_id"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			client, err := p.getClient()
			if err != nil {
				return tool.Result{}, err
			}

			sprints, _, err := client.Board.GetAllSprints(fmt.Sprintf("%d", in.BoardID))
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to list sprints: %w", err)
			}

			result := make([]map[string]any, len(sprints))
			for i, s := range sprints {
				result[i] = map[string]any{
					"id":         s.ID,
					"name":       s.Name,
					"state":      s.State,
					"start_date": s.StartDate,
					"end_date":   s.EndDate,
				}
			}

			output, _ := json.Marshal(map[string]any{
				"count":   len(result),
				"sprints": result,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *jiraPack) getSprintIssuesTool() tool.Tool {
	return tool.NewBuilder("jira_get_sprint_issues").
		WithDescription("Get issues in an active sprint for a board").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				BoardID    int `json:"board_id"`
				MaxResults int `json:"max_results,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.MaxResults == 0 {
				in.MaxResults = 50
			}

			client, err := p.getClient()
			if err != nil {
				return tool.Result{}, err
			}

			// Use JQL to find issues in active sprint for the board
			jql := "sprint in openSprints()"
			issues, _, err := client.Issue.Search(jql, &jira.SearchOptions{
				MaxResults: in.MaxResults,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get sprint issues: %w", err)
			}

			result := make([]map[string]any, len(issues))
			for i, issue := range issues {
				result[i] = formatIssue(&issue)
			}

			output, _ := json.Marshal(map[string]any{
				"count":  len(result),
				"issues": result,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *jiraPack) moveToSprintTool() tool.Tool {
	return tool.NewBuilder("jira_move_to_sprint").
		WithDescription("Move issues to a sprint (updates issue sprint field)").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				SprintID int      `json:"sprint_id"`
				Issues   []string `json:"issues"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			client, err := p.getClient()
			if err != nil {
				return tool.Result{}, err
			}

			// Move each issue by updating the sprint field
			for _, issueKey := range in.Issues {
				update := map[string]interface{}{
					"fields": map[string]interface{}{
						"customfield_10020": in.SprintID, // Common sprint field ID
					},
				}
				_, err = client.Issue.UpdateIssue(issueKey, update)
				if err != nil {
					return tool.Result{}, fmt.Errorf("failed to move issue %s: %w", issueKey, err)
				}
			}

			output, _ := json.Marshal(map[string]any{
				"success": true,
				"count":   len(in.Issues),
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// ============================================================================
// User Tools
// ============================================================================

func (p *jiraPack) getUserTool() tool.Tool {
	return tool.NewBuilder("jira_get_user").
		WithDescription("Get a Jira user by account ID").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				AccountID string `json:"account_id"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			client, err := p.getClient()
			if err != nil {
				return tool.Result{}, err
			}

			user, _, err := client.User.GetByAccountID(in.AccountID)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get user: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"account_id":   user.AccountID,
				"display_name": user.DisplayName,
				"email":        user.EmailAddress,
				"active":       user.Active,
				"timezone":     user.TimeZone,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *jiraPack) searchUsersTool() tool.Tool {
	return tool.NewBuilder("jira_search_users").
		WithDescription("Search for Jira users").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Query      string `json:"query"`
				MaxResults int    `json:"max_results,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.MaxResults == 0 {
				in.MaxResults = 50
			}

			client, err := p.getClient()
			if err != nil {
				return tool.Result{}, err
			}

			users, _, err := client.User.Find(in.Query, jira.WithMaxResults(in.MaxResults))
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to search users: %w", err)
			}

			result := make([]map[string]any, len(users))
			for i, u := range users {
				result[i] = map[string]any{
					"account_id":   u.AccountID,
					"display_name": u.DisplayName,
					"email":        u.EmailAddress,
					"active":       u.Active,
				}
			}

			output, _ := json.Marshal(map[string]any{
				"count": len(result),
				"users": result,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// ============================================================================
// Helpers
// ============================================================================

func formatIssue(issue *jira.Issue) map[string]any {
	result := map[string]any{
		"key":  issue.Key,
		"id":   issue.ID,
		"self": issue.Self,
	}

	if issue.Fields != nil {
		result["summary"] = issue.Fields.Summary
		result["description"] = issue.Fields.Description
		result["created"] = issue.Fields.Created
		result["updated"] = issue.Fields.Updated

		if issue.Fields.Status != nil {
			result["status"] = issue.Fields.Status.Name
		}
		if issue.Fields.Priority != nil {
			result["priority"] = issue.Fields.Priority.Name
		}
		if issue.Fields.Type.Name != "" {
			result["issue_type"] = issue.Fields.Type.Name
		}
		if issue.Fields.Assignee != nil {
			result["assignee"] = issue.Fields.Assignee.DisplayName
		}
		if issue.Fields.Reporter != nil {
			result["reporter"] = issue.Fields.Reporter.DisplayName
		}
		if len(issue.Fields.Labels) > 0 {
			result["labels"] = issue.Fields.Labels
		}
		if issue.Fields.Project.Key != "" {
			result["project"] = issue.Fields.Project.Key
		}
	}

	return result
}
