// Package kubernetes provides Kubernetes operation tools for agent-go.
//
// This pack includes tools for Kubernetes cluster management:
//   - k8s_get: Get a Kubernetes resource
//   - k8s_list: List Kubernetes resources
//   - k8s_apply: Apply a manifest to the cluster
//   - k8s_delete: Delete a Kubernetes resource
//   - k8s_logs: Get pod logs
//   - k8s_exec: Execute a command in a pod
//   - k8s_scale: Scale a deployment or statefulset
//   - k8s_rollout: Manage rollouts (status, restart, undo)
//   - k8s_port_forward: Forward a local port to a pod
//
// Supports kubeconfig-based authentication and in-cluster config.
package kubernetes

import (
	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// Pack returns the Kubernetes tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("kubernetes").
		WithDescription("Kubernetes cluster management tools").
		WithVersion("0.1.0").
		AddTools(
			k8sGet(),
			k8sList(),
			k8sApply(),
			k8sDelete(),
			k8sLogs(),
			k8sExec(),
			k8sScale(),
			k8sRollout(),
			k8sPortForward(),
		).
		AllowInState(agent.StateExplore, "k8s_get", "k8s_list", "k8s_logs").
		AllowInState(agent.StateAct, "k8s_get", "k8s_list", "k8s_apply", "k8s_delete", "k8s_logs", "k8s_exec", "k8s_scale", "k8s_rollout", "k8s_port_forward").
		AllowInState(agent.StateValidate, "k8s_get", "k8s_list", "k8s_logs").
		Build()
}

func k8sGet() tool.Tool {
	return tool.NewBuilder("k8s_get").
		WithDescription("Get a Kubernetes resource by name").
		ReadOnly().
		MustBuild()
}

func k8sList() tool.Tool {
	return tool.NewBuilder("k8s_list").
		WithDescription("List Kubernetes resources with optional label selector").
		ReadOnly().
		Cacheable().
		MustBuild()
}

func k8sApply() tool.Tool {
	return tool.NewBuilder("k8s_apply").
		WithDescription("Apply a manifest to the Kubernetes cluster").
		Idempotent().
		WithRiskLevel(tool.RiskHigh).
		RequiresApproval().
		MustBuild()
}

func k8sDelete() tool.Tool {
	return tool.NewBuilder("k8s_delete").
		WithDescription("Delete a Kubernetes resource").
		Destructive().
		MustBuild()
}

func k8sLogs() tool.Tool {
	return tool.NewBuilder("k8s_logs").
		WithDescription("Get logs from a pod").
		ReadOnly().
		MustBuild()
}

func k8sExec() tool.Tool {
	return tool.NewBuilder("k8s_exec").
		WithDescription("Execute a command in a pod container").
		WithRiskLevel(tool.RiskHigh).
		RequiresApproval().
		MustBuild()
}

func k8sScale() tool.Tool {
	return tool.NewBuilder("k8s_scale").
		WithDescription("Scale a deployment or statefulset").
		WithRiskLevel(tool.RiskMedium).
		MustBuild()
}

func k8sRollout() tool.Tool {
	return tool.NewBuilder("k8s_rollout").
		WithDescription("Manage rollouts: status, restart, or undo").
		WithRiskLevel(tool.RiskMedium).
		MustBuild()
}

func k8sPortForward() tool.Tool {
	return tool.NewBuilder("k8s_port_forward").
		WithDescription("Forward a local port to a pod").
		WithRiskLevel(tool.RiskLow).
		MustBuild()
}
