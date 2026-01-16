// Package kubernetes provides Kubernetes operation tools.
package kubernetes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Config configures the kubernetes pack.
type Config struct {
	// Client is the Kubernetes client (required).
	Client kubernetes.Interface

	// DynamicClient for unstructured operations.
	DynamicClient dynamic.Interface

	// RESTConfig for exec operations.
	RESTConfig *rest.Config

	// Namespace restricts operations to a specific namespace.
	// Empty string means all namespaces.
	Namespace string

	// ReadOnly disables all write operations.
	ReadOnly bool

	// AllowExec enables exec into pods.
	AllowExec bool

	// LogTailLines limits the number of log lines returned.
	LogTailLines int64

	// Timeout for operations.
	Timeout time.Duration
}

// Option configures the kubernetes pack.
type Option func(*Config)

// WithNamespace restricts operations to a specific namespace.
func WithNamespace(ns string) Option {
	return func(c *Config) {
		c.Namespace = ns
	}
}

// WithWriteAccess enables write operations (apply, delete).
func WithWriteAccess() Option {
	return func(c *Config) {
		c.ReadOnly = false
	}
}

// WithExecAccess enables exec into pods.
func WithExecAccess() Option {
	return func(c *Config) {
		c.AllowExec = true
	}
}

// WithLogTailLines sets the number of log lines to return.
func WithLogTailLines(lines int64) Option {
	return func(c *Config) {
		c.LogTailLines = lines
	}
}

// WithTimeout sets the operation timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		c.Timeout = timeout
	}
}

// WithDynamicClient sets the dynamic client for unstructured operations.
func WithDynamicClient(client dynamic.Interface) Option {
	return func(c *Config) {
		c.DynamicClient = client
	}
}

// WithRESTConfig sets the REST config for exec operations.
func WithRESTConfig(config *rest.Config) Option {
	return func(c *Config) {
		c.RESTConfig = config
	}
}

// New creates the kubernetes pack.
func New(client kubernetes.Interface, opts ...Option) (*pack.Pack, error) {
	if client == nil {
		return nil, errors.New("kubernetes client is required")
	}

	cfg := Config{
		Client:       client,
		Namespace:    "",
		ReadOnly:     true, // Read-only by default for safety
		AllowExec:    false,
		LogTailLines: 1000,
		Timeout:      30 * time.Second,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	builder := pack.NewBuilder("kubernetes").
		WithDescription("Kubernetes operations").
		WithVersion("1.0.0").
		AddTools(
			getTool(&cfg),
			listTool(&cfg),
			describeTool(&cfg),
			logsTool(&cfg),
		).
		AllowInState(agent.StateExplore, "k8s_get", "k8s_list", "k8s_describe", "k8s_logs").
		AllowInState(agent.StateValidate, "k8s_get", "k8s_list", "k8s_describe", "k8s_logs")

	// Add write tools if enabled
	if !cfg.ReadOnly {
		builder = builder.AddTools(applyTool(&cfg), deleteTool(&cfg))
		builder = builder.AllowInState(agent.StateAct,
			"k8s_get", "k8s_list", "k8s_describe", "k8s_logs",
			"k8s_apply", "k8s_delete")
	} else {
		builder = builder.AllowInState(agent.StateAct,
			"k8s_get", "k8s_list", "k8s_describe", "k8s_logs")
	}

	// Add exec tool if enabled
	if cfg.AllowExec && !cfg.ReadOnly {
		builder = builder.AddTools(execTool(&cfg))
		// Re-add all tools to Act state including exec
		builder = builder.AllowInState(agent.StateAct,
			"k8s_get", "k8s_list", "k8s_describe", "k8s_logs",
			"k8s_apply", "k8s_delete", "k8s_exec")
	}

	return builder.Build(), nil
}

// getInput is the input for the k8s_get tool.
type getInput struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

// getOutput is the output for the k8s_get tool.
type getOutput struct {
	Kind       string                 `json:"kind"`
	Name       string                 `json:"name"`
	Namespace  string                 `json:"namespace,omitempty"`
	Labels     map[string]string      `json:"labels,omitempty"`
	Status     map[string]interface{} `json:"status,omitempty"`
	Spec       map[string]interface{} `json:"spec,omitempty"`
	APIVersion string                 `json:"api_version"`
}

func getTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("k8s_get").
		WithDescription("Get a Kubernetes resource by name").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in getInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.Kind == "" {
				return tool.Result{}, errors.New("kind is required")
			}
			if in.Name == "" {
				return tool.Result{}, errors.New("name is required")
			}

			ns := resolveNamespace(cfg.Namespace, in.Namespace)

			ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			out, err := getResource(ctx, cfg, in.Kind, in.Name, ns)
			if err != nil {
				return tool.Result{}, err
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

// listInput is the input for the k8s_list tool.
type listInput struct {
	Kind          string `json:"kind"`
	Namespace     string `json:"namespace,omitempty"`
	LabelSelector string `json:"label_selector,omitempty"`
	FieldSelector string `json:"field_selector,omitempty"`
	Limit         int64  `json:"limit,omitempty"`
}

// listOutput is the output for the k8s_list tool.
type listOutput struct {
	Kind  string       `json:"kind"`
	Items []listItem   `json:"items"`
	Count int          `json:"count"`
}

type listItem struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	Status    string            `json:"status,omitempty"`
	Age       string            `json:"age,omitempty"`
}

func listTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("k8s_list").
		WithDescription("List Kubernetes resources").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in listInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.Kind == "" {
				return tool.Result{}, errors.New("kind is required")
			}

			ns := resolveNamespace(cfg.Namespace, in.Namespace)
			if in.Limit == 0 {
				in.Limit = 100
			}

			ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			out, err := listResources(ctx, cfg, in.Kind, ns, in.LabelSelector, in.FieldSelector, in.Limit)
			if err != nil {
				return tool.Result{}, err
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

// describeInput is the input for the k8s_describe tool.
type describeInput struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

// describeOutput is the output for the k8s_describe tool.
type describeOutput struct {
	Kind        string                 `json:"kind"`
	Name        string                 `json:"name"`
	Namespace   string                 `json:"namespace,omitempty"`
	Labels      map[string]string      `json:"labels,omitempty"`
	Annotations map[string]string      `json:"annotations,omitempty"`
	Status      map[string]interface{} `json:"status,omitempty"`
	Spec        map[string]interface{} `json:"spec,omitempty"`
	Events      []eventInfo            `json:"events,omitempty"`
	Conditions  []conditionInfo        `json:"conditions,omitempty"`
}

type eventInfo struct {
	Type    string `json:"type"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
	Age     string `json:"age"`
	Count   int32  `json:"count"`
}

type conditionInfo struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

func describeTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("k8s_describe").
		WithDescription("Describe a Kubernetes resource with events and conditions").
		ReadOnly().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in describeInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.Kind == "" {
				return tool.Result{}, errors.New("kind is required")
			}
			if in.Name == "" {
				return tool.Result{}, errors.New("name is required")
			}

			ns := resolveNamespace(cfg.Namespace, in.Namespace)

			ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			out, err := describeResource(ctx, cfg, in.Kind, in.Name, ns)
			if err != nil {
				return tool.Result{}, err
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

// logsInput is the input for the k8s_logs tool.
type logsInput struct {
	Pod       string `json:"pod"`
	Namespace string `json:"namespace,omitempty"`
	Container string `json:"container,omitempty"`
	TailLines int64  `json:"tail_lines,omitempty"`
	Previous  bool   `json:"previous,omitempty"`
}

// logsOutput is the output for the k8s_logs tool.
type logsOutput struct {
	Pod       string `json:"pod"`
	Container string `json:"container,omitempty"`
	Logs      string `json:"logs"`
	LineCount int    `json:"line_count"`
	Truncated bool   `json:"truncated,omitempty"`
}

func logsTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("k8s_logs").
		WithDescription("Get logs from a pod").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in logsInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.Pod == "" {
				return tool.Result{}, errors.New("pod name is required")
			}

			ns := resolveNamespace(cfg.Namespace, in.Namespace)
			if ns == "" {
				ns = "default"
			}

			tailLines := in.TailLines
			if tailLines == 0 {
				tailLines = cfg.LogTailLines
			}

			ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			opts := &corev1.PodLogOptions{
				Container: in.Container,
				TailLines: &tailLines,
				Previous:  in.Previous,
			}

			req := cfg.Client.CoreV1().Pods(ns).GetLogs(in.Pod, opts)
			stream, err := req.Stream(ctx)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get logs: %w", err)
			}
			defer stream.Close()

			var buf bytes.Buffer
			_, err = io.Copy(&buf, stream)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to read logs: %w", err)
			}

			logs := buf.String()
			lines := strings.Count(logs, "\n")

			out := logsOutput{
				Pod:       in.Pod,
				Container: in.Container,
				Logs:      logs,
				LineCount: lines,
				Truncated: int64(lines) >= tailLines,
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

// applyInput is the input for the k8s_apply tool.
type applyInput struct {
	Manifest  string `json:"manifest"`
	Namespace string `json:"namespace,omitempty"`
	DryRun    bool   `json:"dry_run,omitempty"`
}

// applyOutput is the output for the k8s_apply tool.
type applyOutput struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	Action    string `json:"action"` // created, configured, unchanged
	DryRun    bool   `json:"dry_run,omitempty"`
}

func applyTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("k8s_apply").
		WithDescription("Apply a Kubernetes manifest").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in applyInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.Manifest == "" {
				return tool.Result{}, errors.New("manifest is required")
			}

			if cfg.DynamicClient == nil {
				return tool.Result{}, errors.New("dynamic client is required for apply operations")
			}

			ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			// Parse the manifest
			decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(in.Manifest), 4096)
			var obj unstructured.Unstructured
			if err := decoder.Decode(&obj); err != nil {
				return tool.Result{}, fmt.Errorf("failed to parse manifest: %w", err)
			}

			ns := resolveNamespace(cfg.Namespace, in.Namespace)
			if ns == "" && obj.GetNamespace() != "" {
				ns = obj.GetNamespace()
			}
			if ns == "" {
				ns = "default"
			}

			gvk := obj.GroupVersionKind()
			gvr := schema.GroupVersionResource{
				Group:    gvk.Group,
				Version:  gvk.Version,
				Resource: strings.ToLower(gvk.Kind) + "s", // Simple pluralization
			}

			var dryRunOpts []string
			if in.DryRun {
				dryRunOpts = []string{metav1.DryRunAll}
			}

			// Try to get existing resource
			existing, err := cfg.DynamicClient.Resource(gvr).Namespace(ns).Get(ctx, obj.GetName(), metav1.GetOptions{})

			var action string
			if err != nil {
				// Create new resource
				_, err = cfg.DynamicClient.Resource(gvr).Namespace(ns).Create(ctx, &obj, metav1.CreateOptions{
					DryRun: dryRunOpts,
				})
				if err != nil {
					return tool.Result{}, fmt.Errorf("failed to create resource: %w", err)
				}
				action = "created"
			} else {
				// Update existing resource
				obj.SetResourceVersion(existing.GetResourceVersion())
				_, err = cfg.DynamicClient.Resource(gvr).Namespace(ns).Update(ctx, &obj, metav1.UpdateOptions{
					DryRun: dryRunOpts,
				})
				if err != nil {
					// Try patch instead
					patchData, _ := json.Marshal(obj.Object)
					_, err = cfg.DynamicClient.Resource(gvr).Namespace(ns).Patch(ctx, obj.GetName(),
						types.MergePatchType, patchData, metav1.PatchOptions{
							DryRun: dryRunOpts,
						})
					if err != nil {
						return tool.Result{}, fmt.Errorf("failed to update resource: %w", err)
					}
				}
				action = "configured"
			}

			out := applyOutput{
				Kind:      gvk.Kind,
				Name:      obj.GetName(),
				Namespace: ns,
				Action:    action,
				DryRun:    in.DryRun,
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

// deleteInput is the input for the k8s_delete tool.
type deleteInput struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	DryRun    bool   `json:"dry_run,omitempty"`
}

// deleteOutput is the output for the k8s_delete tool.
type deleteOutput struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	Deleted   bool   `json:"deleted"`
	DryRun    bool   `json:"dry_run,omitempty"`
}

func deleteTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("k8s_delete").
		WithDescription("Delete a Kubernetes resource").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in deleteInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.Kind == "" {
				return tool.Result{}, errors.New("kind is required")
			}
			if in.Name == "" {
				return tool.Result{}, errors.New("name is required")
			}

			ns := resolveNamespace(cfg.Namespace, in.Namespace)

			ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			var dryRunOpts []string
			if in.DryRun {
				dryRunOpts = []string{metav1.DryRunAll}
			}

			err := deleteResource(ctx, cfg, in.Kind, in.Name, ns, dryRunOpts)
			if err != nil {
				return tool.Result{}, err
			}

			out := deleteOutput{
				Kind:      in.Kind,
				Name:      in.Name,
				Namespace: ns,
				Deleted:   true,
				DryRun:    in.DryRun,
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

// execInput is the input for the k8s_exec tool.
type execInput struct {
	Pod       string   `json:"pod"`
	Namespace string   `json:"namespace,omitempty"`
	Container string   `json:"container,omitempty"`
	Command   []string `json:"command"`
}

// execOutput is the output for the k8s_exec tool.
type execOutput struct {
	Pod       string `json:"pod"`
	Container string `json:"container,omitempty"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	ExitCode  int    `json:"exit_code"`
}

func execTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("k8s_exec").
		WithDescription("Execute a command in a pod").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in execInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.Pod == "" {
				return tool.Result{}, errors.New("pod name is required")
			}
			if len(in.Command) == 0 {
				return tool.Result{}, errors.New("command is required")
			}

			if cfg.RESTConfig == nil {
				return tool.Result{}, errors.New("REST config is required for exec operations")
			}

			ns := resolveNamespace(cfg.Namespace, in.Namespace)
			if ns == "" {
				ns = "default"
			}

			// Note: Full exec implementation requires remotecommand package
			// This is a simplified version that returns an error for now
			// Real implementation would use:
			// - k8s.io/client-go/tools/remotecommand
			// - SPDYExecutor for streaming
			return tool.Result{}, errors.New("exec not fully implemented - requires SPDY executor setup")
		}).
		MustBuild()
}

// Helper functions

func resolveNamespace(configNS, inputNS string) string {
	if configNS != "" {
		return configNS // Config namespace takes precedence
	}
	return inputNS
}

func getResource(ctx context.Context, cfg *Config, kind, name, ns string) (*getOutput, error) {
	kind = strings.ToLower(kind)

	switch kind {
	case "pod", "pods":
		pod, err := cfg.Client.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get pod: %w", err)
		}
		return &getOutput{
			Kind:       "Pod",
			Name:       pod.Name,
			Namespace:  pod.Namespace,
			Labels:     pod.Labels,
			APIVersion: "v1",
			Status:     podStatusToMap(pod),
			Spec:       podSpecToMap(pod),
		}, nil

	case "deployment", "deployments":
		deploy, err := cfg.Client.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get deployment: %w", err)
		}
		return &getOutput{
			Kind:       "Deployment",
			Name:       deploy.Name,
			Namespace:  deploy.Namespace,
			Labels:     deploy.Labels,
			APIVersion: "apps/v1",
			Status: map[string]interface{}{
				"replicas":          deploy.Status.Replicas,
				"ready_replicas":    deploy.Status.ReadyReplicas,
				"available_replicas": deploy.Status.AvailableReplicas,
			},
		}, nil

	case "service", "services":
		svc, err := cfg.Client.CoreV1().Services(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get service: %w", err)
		}
		return &getOutput{
			Kind:       "Service",
			Name:       svc.Name,
			Namespace:  svc.Namespace,
			Labels:     svc.Labels,
			APIVersion: "v1",
			Spec: map[string]interface{}{
				"type":       string(svc.Spec.Type),
				"cluster_ip": svc.Spec.ClusterIP,
				"ports":      svc.Spec.Ports,
			},
		}, nil

	case "configmap", "configmaps":
		cm, err := cfg.Client.CoreV1().ConfigMaps(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get configmap: %w", err)
		}
		return &getOutput{
			Kind:       "ConfigMap",
			Name:       cm.Name,
			Namespace:  cm.Namespace,
			Labels:     cm.Labels,
			APIVersion: "v1",
			Spec: map[string]interface{}{
				"data": cm.Data,
			},
		}, nil

	case "secret", "secrets":
		secret, err := cfg.Client.CoreV1().Secrets(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get secret: %w", err)
		}
		// Don't expose secret data, only metadata
		return &getOutput{
			Kind:       "Secret",
			Name:       secret.Name,
			Namespace:  secret.Namespace,
			Labels:     secret.Labels,
			APIVersion: "v1",
			Spec: map[string]interface{}{
				"type": string(secret.Type),
				"keys": getSecretKeys(secret),
			},
		}, nil

	case "namespace", "namespaces":
		namespace, err := cfg.Client.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get namespace: %w", err)
		}
		return &getOutput{
			Kind:       "Namespace",
			Name:       namespace.Name,
			Labels:     namespace.Labels,
			APIVersion: "v1",
			Status: map[string]interface{}{
				"phase": string(namespace.Status.Phase),
			},
		}, nil

	default:
		return nil, fmt.Errorf("unsupported resource kind: %s", kind)
	}
}

func listResources(ctx context.Context, cfg *Config, kind, ns, labelSelector, fieldSelector string, limit int64) (*listOutput, error) {
	kind = strings.ToLower(kind)
	opts := metav1.ListOptions{
		LabelSelector: labelSelector,
		FieldSelector: fieldSelector,
		Limit:         limit,
	}

	switch kind {
	case "pod", "pods":
		pods, err := cfg.Client.CoreV1().Pods(ns).List(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list pods: %w", err)
		}
		items := make([]listItem, 0, len(pods.Items))
		for _, pod := range pods.Items {
			items = append(items, listItem{
				Name:      pod.Name,
				Namespace: pod.Namespace,
				Labels:    pod.Labels,
				Status:    string(pod.Status.Phase),
				Age:       formatAge(pod.CreationTimestamp.Time),
			})
		}
		return &listOutput{Kind: "Pod", Items: items, Count: len(items)}, nil

	case "deployment", "deployments":
		deploys, err := cfg.Client.AppsV1().Deployments(ns).List(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list deployments: %w", err)
		}
		items := make([]listItem, 0, len(deploys.Items))
		for _, d := range deploys.Items {
			status := fmt.Sprintf("%d/%d", d.Status.ReadyReplicas, d.Status.Replicas)
			items = append(items, listItem{
				Name:      d.Name,
				Namespace: d.Namespace,
				Labels:    d.Labels,
				Status:    status,
				Age:       formatAge(d.CreationTimestamp.Time),
			})
		}
		return &listOutput{Kind: "Deployment", Items: items, Count: len(items)}, nil

	case "service", "services":
		svcs, err := cfg.Client.CoreV1().Services(ns).List(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list services: %w", err)
		}
		items := make([]listItem, 0, len(svcs.Items))
		for _, svc := range svcs.Items {
			items = append(items, listItem{
				Name:      svc.Name,
				Namespace: svc.Namespace,
				Labels:    svc.Labels,
				Status:    string(svc.Spec.Type),
				Age:       formatAge(svc.CreationTimestamp.Time),
			})
		}
		return &listOutput{Kind: "Service", Items: items, Count: len(items)}, nil

	case "namespace", "namespaces":
		namespaces, err := cfg.Client.CoreV1().Namespaces().List(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list namespaces: %w", err)
		}
		items := make([]listItem, 0, len(namespaces.Items))
		for _, ns := range namespaces.Items {
			items = append(items, listItem{
				Name:   ns.Name,
				Labels: ns.Labels,
				Status: string(ns.Status.Phase),
				Age:    formatAge(ns.CreationTimestamp.Time),
			})
		}
		return &listOutput{Kind: "Namespace", Items: items, Count: len(items)}, nil

	case "configmap", "configmaps":
		cms, err := cfg.Client.CoreV1().ConfigMaps(ns).List(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list configmaps: %w", err)
		}
		items := make([]listItem, 0, len(cms.Items))
		for _, cm := range cms.Items {
			items = append(items, listItem{
				Name:      cm.Name,
				Namespace: cm.Namespace,
				Labels:    cm.Labels,
				Status:    fmt.Sprintf("%d keys", len(cm.Data)),
				Age:       formatAge(cm.CreationTimestamp.Time),
			})
		}
		return &listOutput{Kind: "ConfigMap", Items: items, Count: len(items)}, nil

	case "secret", "secrets":
		secrets, err := cfg.Client.CoreV1().Secrets(ns).List(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list secrets: %w", err)
		}
		items := make([]listItem, 0, len(secrets.Items))
		for _, s := range secrets.Items {
			items = append(items, listItem{
				Name:      s.Name,
				Namespace: s.Namespace,
				Labels:    s.Labels,
				Status:    string(s.Type),
				Age:       formatAge(s.CreationTimestamp.Time),
			})
		}
		return &listOutput{Kind: "Secret", Items: items, Count: len(items)}, nil

	default:
		return nil, fmt.Errorf("unsupported resource kind: %s", kind)
	}
}

func describeResource(ctx context.Context, cfg *Config, kind, name, ns string) (*describeOutput, error) {
	kind = strings.ToLower(kind)

	switch kind {
	case "pod", "pods":
		pod, err := cfg.Client.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get pod: %w", err)
		}

		// Get events for this pod
		events, _ := cfg.Client.CoreV1().Events(ns).List(ctx, metav1.ListOptions{
			FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Pod", name),
		})

		eventInfos := make([]eventInfo, 0)
		if events != nil {
			for _, e := range events.Items {
				eventInfos = append(eventInfos, eventInfo{
					Type:    e.Type,
					Reason:  e.Reason,
					Message: e.Message,
					Age:     formatAge(e.LastTimestamp.Time),
					Count:   e.Count,
				})
			}
		}

		conditions := make([]conditionInfo, 0, len(pod.Status.Conditions))
		for _, c := range pod.Status.Conditions {
			conditions = append(conditions, conditionInfo{
				Type:    string(c.Type),
				Status:  string(c.Status),
				Reason:  c.Reason,
				Message: c.Message,
			})
		}

		return &describeOutput{
			Kind:        "Pod",
			Name:        pod.Name,
			Namespace:   pod.Namespace,
			Labels:      pod.Labels,
			Annotations: pod.Annotations,
			Status:      podStatusToMap(pod),
			Spec:        podSpecToMap(pod),
			Events:      eventInfos,
			Conditions:  conditions,
		}, nil

	case "deployment", "deployments":
		deploy, err := cfg.Client.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get deployment: %w", err)
		}

		conditions := make([]conditionInfo, 0, len(deploy.Status.Conditions))
		for _, c := range deploy.Status.Conditions {
			conditions = append(conditions, conditionInfo{
				Type:    string(c.Type),
				Status:  string(c.Status),
				Reason:  c.Reason,
				Message: c.Message,
			})
		}

		return &describeOutput{
			Kind:        "Deployment",
			Name:        deploy.Name,
			Namespace:   deploy.Namespace,
			Labels:      deploy.Labels,
			Annotations: deploy.Annotations,
			Status: map[string]interface{}{
				"replicas":           deploy.Status.Replicas,
				"ready_replicas":     deploy.Status.ReadyReplicas,
				"available_replicas": deploy.Status.AvailableReplicas,
				"updated_replicas":   deploy.Status.UpdatedReplicas,
			},
			Conditions: conditions,
		}, nil

	default:
		return nil, fmt.Errorf("describe not supported for kind: %s", kind)
	}
}

func deleteResource(ctx context.Context, cfg *Config, kind, name, ns string, dryRunOpts []string) error {
	kind = strings.ToLower(kind)
	deleteOpts := metav1.DeleteOptions{
		DryRun: dryRunOpts,
	}

	switch kind {
	case "pod", "pods":
		return cfg.Client.CoreV1().Pods(ns).Delete(ctx, name, deleteOpts)
	case "deployment", "deployments":
		return cfg.Client.AppsV1().Deployments(ns).Delete(ctx, name, deleteOpts)
	case "service", "services":
		return cfg.Client.CoreV1().Services(ns).Delete(ctx, name, deleteOpts)
	case "configmap", "configmaps":
		return cfg.Client.CoreV1().ConfigMaps(ns).Delete(ctx, name, deleteOpts)
	case "secret", "secrets":
		return cfg.Client.CoreV1().Secrets(ns).Delete(ctx, name, deleteOpts)
	case "namespace", "namespaces":
		return cfg.Client.CoreV1().Namespaces().Delete(ctx, name, deleteOpts)
	default:
		return fmt.Errorf("delete not supported for kind: %s", kind)
	}
}

func podStatusToMap(pod *corev1.Pod) map[string]interface{} {
	containerStatuses := make([]map[string]interface{}, 0, len(pod.Status.ContainerStatuses))
	for _, cs := range pod.Status.ContainerStatuses {
		containerStatuses = append(containerStatuses, map[string]interface{}{
			"name":          cs.Name,
			"ready":         cs.Ready,
			"restart_count": cs.RestartCount,
			"image":         cs.Image,
		})
	}

	return map[string]interface{}{
		"phase":              string(pod.Status.Phase),
		"host_ip":            pod.Status.HostIP,
		"pod_ip":             pod.Status.PodIP,
		"start_time":         pod.Status.StartTime,
		"container_statuses": containerStatuses,
	}
}

func podSpecToMap(pod *corev1.Pod) map[string]interface{} {
	containers := make([]map[string]interface{}, 0, len(pod.Spec.Containers))
	for _, c := range pod.Spec.Containers {
		containers = append(containers, map[string]interface{}{
			"name":  c.Name,
			"image": c.Image,
			"ports": c.Ports,
		})
	}

	return map[string]interface{}{
		"node_name":       pod.Spec.NodeName,
		"service_account": pod.Spec.ServiceAccountName,
		"containers":      containers,
	}
}

func getSecretKeys(secret *corev1.Secret) []string {
	keys := make([]string, 0, len(secret.Data))
	for k := range secret.Data {
		keys = append(keys, k)
	}
	return keys
}

func formatAge(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	d := time.Since(t)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}
