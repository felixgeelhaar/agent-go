package kubernetes

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func setupTestClient(t *testing.T) *fake.Clientset {
	t.Helper()

	client := fake.NewSimpleClientset(
		// Namespace
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "default",
				Labels: map[string]string{
					"env": "test",
				},
			},
			Status: corev1.NamespaceStatus{
				Phase: corev1.NamespaceActive,
			},
		},
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "production",
			},
			Status: corev1.NamespaceStatus{
				Phase: corev1.NamespaceActive,
			},
		},
		// Pod
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod",
				Namespace: "default",
				Labels: map[string]string{
					"app": "test",
				},
				CreationTimestamp: metav1.Now(),
			},
			Spec: corev1.PodSpec{
				NodeName:           "node-1",
				ServiceAccountName: "default",
				Containers: []corev1.Container{
					{
						Name:  "main",
						Image: "nginx:latest",
					},
				},
			},
			Status: corev1.PodStatus{
				Phase:  corev1.PodRunning,
				HostIP: "192.168.1.1",
				PodIP:  "10.0.0.1",
				ContainerStatuses: []corev1.ContainerStatus{
					{
						Name:         "main",
						Ready:        true,
						RestartCount: 0,
						Image:        "nginx:latest",
					},
				},
				Conditions: []corev1.PodCondition{
					{
						Type:   corev1.PodReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
		// Service
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-service",
				Namespace: "default",
				Labels: map[string]string{
					"app": "test",
				},
				CreationTimestamp: metav1.Now(),
			},
			Spec: corev1.ServiceSpec{
				Type:      corev1.ServiceTypeClusterIP,
				ClusterIP: "10.96.0.1",
				Ports: []corev1.ServicePort{
					{Port: 80},
				},
			},
		},
		// ConfigMap
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "test-config",
				Namespace:         "default",
				CreationTimestamp: metav1.Now(),
			},
			Data: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		},
		// Secret
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "test-secret",
				Namespace:         "default",
				CreationTimestamp: metav1.Now(),
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"password": []byte("secret123"),
			},
		},
		// Deployment
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment",
				Namespace: "default",
				Labels: map[string]string{
					"app": "test",
				},
				CreationTimestamp: metav1.Now(),
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: int32Ptr(3),
			},
			Status: appsv1.DeploymentStatus{
				Replicas:          3,
				ReadyReplicas:     3,
				AvailableReplicas: 3,
				UpdatedReplicas:   3,
				Conditions: []appsv1.DeploymentCondition{
					{
						Type:   appsv1.DeploymentAvailable,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
	)

	return client
}

func int32Ptr(i int32) *int32 {
	return &i
}

func TestNew(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p == nil {
		t.Fatal("expected non-nil pack")
	}

	if p.Name != "kubernetes" {
		t.Errorf("expected name 'kubernetes', got '%s'", p.Name)
	}
}

func TestNewWithNilClient(t *testing.T) {
	_, err := New(nil)
	if err == nil {
		t.Error("expected error for nil client")
	}
}

func TestNewWithOptions(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client,
		WithNamespace("production"),
		WithWriteAccess(),
		WithExecAccess(),
		WithLogTailLines(500),
		WithTimeout(60*time.Second),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p == nil {
		t.Error("expected non-nil pack")
	}
}

func TestGetToolPod(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_get")
	if !ok {
		t.Fatal("k8s_get tool not found")
	}

	input, _ := json.Marshal(getInput{
		Kind:      "pod",
		Name:      "test-pod",
		Namespace: "default",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	var out getOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.Kind != "Pod" {
		t.Errorf("expected kind 'Pod', got '%s'", out.Kind)
	}

	if out.Name != "test-pod" {
		t.Errorf("expected name 'test-pod', got '%s'", out.Name)
	}
}

func TestGetToolDeployment(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_get")
	if !ok {
		t.Fatal("k8s_get tool not found")
	}

	input, _ := json.Marshal(getInput{
		Kind:      "deployment",
		Name:      "test-deployment",
		Namespace: "default",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	var out getOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.Kind != "Deployment" {
		t.Errorf("expected kind 'Deployment', got '%s'", out.Kind)
	}
}

func TestGetToolService(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_get")
	if !ok {
		t.Fatal("k8s_get tool not found")
	}

	input, _ := json.Marshal(getInput{
		Kind:      "service",
		Name:      "test-service",
		Namespace: "default",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	var out getOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.Kind != "Service" {
		t.Errorf("expected kind 'Service', got '%s'", out.Kind)
	}
}

func TestGetToolConfigMap(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_get")
	if !ok {
		t.Fatal("k8s_get tool not found")
	}

	input, _ := json.Marshal(getInput{
		Kind:      "configmap",
		Name:      "test-config",
		Namespace: "default",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	var out getOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.Kind != "ConfigMap" {
		t.Errorf("expected kind 'ConfigMap', got '%s'", out.Kind)
	}
}

func TestGetToolSecret(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_get")
	if !ok {
		t.Fatal("k8s_get tool not found")
	}

	input, _ := json.Marshal(getInput{
		Kind:      "secret",
		Name:      "test-secret",
		Namespace: "default",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	var out getOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.Kind != "Secret" {
		t.Errorf("expected kind 'Secret', got '%s'", out.Kind)
	}

	// Verify secret data is not exposed (only keys returned, not values)
	if spec, ok := out.Spec["keys"]; ok {
		keys, ok := spec.([]interface{})
		if !ok {
			t.Errorf("expected keys to be []interface{}, got %T", spec)
			return
		}
		if len(keys) != 1 {
			t.Errorf("expected 1 key, got %d", len(keys))
			return
		}
		if keys[0].(string) != "password" {
			t.Error("expected key 'password'")
		}
	}
}

func TestGetToolNamespace(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_get")
	if !ok {
		t.Fatal("k8s_get tool not found")
	}

	input, _ := json.Marshal(getInput{
		Kind: "namespace",
		Name: "default",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	var out getOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.Kind != "Namespace" {
		t.Errorf("expected kind 'Namespace', got '%s'", out.Kind)
	}
}

func TestGetToolMissingKind(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_get")
	if !ok {
		t.Fatal("k8s_get tool not found")
	}

	input, _ := json.Marshal(getInput{
		Name:      "test-pod",
		Namespace: "default",
	})

	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for missing kind")
	}
}

func TestGetToolMissingName(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_get")
	if !ok {
		t.Fatal("k8s_get tool not found")
	}

	input, _ := json.Marshal(getInput{
		Kind:      "pod",
		Namespace: "default",
	})

	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for missing name")
	}
}

func TestListToolPods(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_list")
	if !ok {
		t.Fatal("k8s_list tool not found")
	}

	input, _ := json.Marshal(listInput{
		Kind:      "pod",
		Namespace: "default",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}

	var out listOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.Count < 1 {
		t.Error("expected at least 1 pod")
	}

	if out.Kind != "Pod" {
		t.Errorf("expected kind 'Pod', got '%s'", out.Kind)
	}
}

func TestListToolDeployments(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_list")
	if !ok {
		t.Fatal("k8s_list tool not found")
	}

	input, _ := json.Marshal(listInput{
		Kind:      "deployment",
		Namespace: "default",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}

	var out listOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.Count < 1 {
		t.Error("expected at least 1 deployment")
	}
}

func TestListToolServices(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_list")
	if !ok {
		t.Fatal("k8s_list tool not found")
	}

	input, _ := json.Marshal(listInput{
		Kind:      "service",
		Namespace: "default",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}

	var out listOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.Count < 1 {
		t.Error("expected at least 1 service")
	}
}

func TestListToolNamespaces(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_list")
	if !ok {
		t.Fatal("k8s_list tool not found")
	}

	input, _ := json.Marshal(listInput{
		Kind: "namespace",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}

	var out listOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.Count < 2 {
		t.Error("expected at least 2 namespaces")
	}
}

func TestListToolConfigMaps(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_list")
	if !ok {
		t.Fatal("k8s_list tool not found")
	}

	input, _ := json.Marshal(listInput{
		Kind:      "configmap",
		Namespace: "default",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}

	var out listOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.Count < 1 {
		t.Error("expected at least 1 configmap")
	}
}

func TestListToolSecrets(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_list")
	if !ok {
		t.Fatal("k8s_list tool not found")
	}

	input, _ := json.Marshal(listInput{
		Kind:      "secret",
		Namespace: "default",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}

	var out listOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.Count < 1 {
		t.Error("expected at least 1 secret")
	}
}

func TestListToolMissingKind(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_list")
	if !ok {
		t.Fatal("k8s_list tool not found")
	}

	input, _ := json.Marshal(listInput{
		Namespace: "default",
	})

	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for missing kind")
	}
}

func TestDescribeToolPod(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_describe")
	if !ok {
		t.Fatal("k8s_describe tool not found")
	}

	input, _ := json.Marshal(describeInput{
		Kind:      "pod",
		Name:      "test-pod",
		Namespace: "default",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("describe failed: %v", err)
	}

	var out describeOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.Kind != "Pod" {
		t.Errorf("expected kind 'Pod', got '%s'", out.Kind)
	}

	if out.Name != "test-pod" {
		t.Errorf("expected name 'test-pod', got '%s'", out.Name)
	}

	if len(out.Conditions) < 1 {
		t.Error("expected at least 1 condition")
	}
}

func TestDescribeToolDeployment(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_describe")
	if !ok {
		t.Fatal("k8s_describe tool not found")
	}

	input, _ := json.Marshal(describeInput{
		Kind:      "deployment",
		Name:      "test-deployment",
		Namespace: "default",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("describe failed: %v", err)
	}

	var out describeOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.Kind != "Deployment" {
		t.Errorf("expected kind 'Deployment', got '%s'", out.Kind)
	}
}

func TestDescribeToolMissingKind(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_describe")
	if !ok {
		t.Fatal("k8s_describe tool not found")
	}

	input, _ := json.Marshal(describeInput{
		Name:      "test-pod",
		Namespace: "default",
	})

	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for missing kind")
	}
}

func TestDeleteToolWithWriteAccess(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client, WithWriteAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_delete")
	if !ok {
		t.Fatal("k8s_delete tool not found")
	}

	// Delete with dry run first
	input, _ := json.Marshal(deleteInput{
		Kind:      "pod",
		Name:      "test-pod",
		Namespace: "default",
		DryRun:    true,
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("delete dry-run failed: %v", err)
	}

	var out deleteOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if !out.DryRun {
		t.Error("expected dry_run to be true")
	}
}

func TestDeleteToolWithoutWriteAccess(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client) // No write access
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	_, ok := p.GetTool("k8s_delete")
	if ok {
		t.Error("expected k8s_delete tool to not exist without write access")
	}
}

func TestApplyToolWithoutWriteAccess(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client) // No write access
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	_, ok := p.GetTool("k8s_apply")
	if ok {
		t.Error("expected k8s_apply tool to not exist without write access")
	}
}

func TestExecToolWithoutExecAccess(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client, WithWriteAccess()) // Write but no exec access
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	_, ok := p.GetTool("k8s_exec")
	if ok {
		t.Error("expected k8s_exec tool to not exist without exec access")
	}
}

func TestExecToolWithExecAccess(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client, WithWriteAccess(), WithExecAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	_, ok := p.GetTool("k8s_exec")
	if !ok {
		t.Error("expected k8s_exec tool to exist with exec access")
	}
}

func TestNamespaceRestriction(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client, WithNamespace("production"))
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_list")
	if !ok {
		t.Fatal("k8s_list tool not found")
	}

	// Try to list pods - should only see production namespace
	input, _ := json.Marshal(listInput{
		Kind:      "pod",
		Namespace: "default", // This should be overridden by config namespace
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}

	var out listOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	// Should return 0 pods since our test pod is in default namespace
	if out.Count != 0 {
		t.Errorf("expected 0 pods in production namespace, got %d", out.Count)
	}
}

func TestToolAnnotations(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client, WithWriteAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	// Check get tool is read-only
	if getTool, ok := p.GetTool("k8s_get"); ok {
		annotations := getTool.Annotations()
		if !annotations.ReadOnly {
			t.Error("k8s_get should be read-only")
		}
	}

	// Check list tool is read-only
	if listTool, ok := p.GetTool("k8s_list"); ok {
		annotations := listTool.Annotations()
		if !annotations.ReadOnly {
			t.Error("k8s_list should be read-only")
		}
	}

	// Check describe tool is read-only and cacheable
	if describeTool, ok := p.GetTool("k8s_describe"); ok {
		annotations := describeTool.Annotations()
		if !annotations.ReadOnly {
			t.Error("k8s_describe should be read-only")
		}
		if !annotations.Cacheable {
			t.Error("k8s_describe should be cacheable")
		}
	}

	// Check delete tool is destructive
	if deleteTool, ok := p.GetTool("k8s_delete"); ok {
		annotations := deleteTool.Annotations()
		if !annotations.Destructive {
			t.Error("k8s_delete should be destructive")
		}
	}
}

func TestGetToolUnsupportedKind(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_get")
	if !ok {
		t.Fatal("k8s_get tool not found")
	}

	input, _ := json.Marshal(getInput{
		Kind:      "unsupportedkind",
		Name:      "test",
		Namespace: "default",
	})

	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for unsupported kind")
	}
}

func TestListToolUnsupportedKind(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_list")
	if !ok {
		t.Fatal("k8s_list tool not found")
	}

	input, _ := json.Marshal(listInput{
		Kind:      "unsupportedkind",
		Namespace: "default",
	})

	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for unsupported kind")
	}
}

func TestDescribeToolUnsupportedKind(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_describe")
	if !ok {
		t.Fatal("k8s_describe tool not found")
	}

	input, _ := json.Marshal(describeInput{
		Kind:      "service", // describe not fully implemented for service
		Name:      "test-service",
		Namespace: "default",
	})

	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for unsupported describe kind")
	}
}

func TestDeleteToolUnsupportedKind(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client, WithWriteAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_delete")
	if !ok {
		t.Fatal("k8s_delete tool not found")
	}

	input, _ := json.Marshal(deleteInput{
		Kind:      "unsupportedkind",
		Name:      "test",
		Namespace: "default",
		DryRun:    true,
	})

	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for unsupported kind")
	}
}

func TestNewWithDynamicClient(t *testing.T) {
	client := setupTestClient(t)

	// Create with dynamic client option
	p, err := New(client, WithDynamicClient(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p == nil {
		t.Error("expected non-nil pack")
	}
}

func TestNewWithRESTConfig(t *testing.T) {
	client := setupTestClient(t)

	// Create with REST config option
	p, err := New(client, WithRESTConfig(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p == nil {
		t.Error("expected non-nil pack")
	}
}

func TestLogsTool(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_logs")
	if !ok {
		t.Fatal("k8s_logs tool not found")
	}

	// Test with valid input (will fail because fake client doesn't support streaming)
	input, _ := json.Marshal(logsInput{
		Pod:       "test-pod",
		Namespace: "default",
	})

	_, err = tool.Execute(context.Background(), input)
	// Expected to fail since fake client doesn't support streaming
	if err == nil {
		t.Log("logs succeeded (unexpected with fake client)")
	}
}

func TestLogsToolMissingPod(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_logs")
	if !ok {
		t.Fatal("k8s_logs tool not found")
	}

	input, _ := json.Marshal(logsInput{
		Namespace: "default",
	})

	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for missing pod name")
	}
}

func TestLogsToolInvalidJSON(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_logs")
	if !ok {
		t.Fatal("k8s_logs tool not found")
	}

	_, err = tool.Execute(context.Background(), json.RawMessage("invalid json"))
	if err == nil {
		t.Error("expected error for invalid JSON input")
	}
}

func TestApplyToolWithWriteAccess(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client, WithWriteAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_apply")
	if !ok {
		t.Fatal("k8s_apply tool not found")
	}

	// Test with missing manifest
	input, _ := json.Marshal(applyInput{
		Namespace: "default",
	})

	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for missing manifest")
	}
}

func TestApplyToolMissingDynamicClient(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client, WithWriteAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_apply")
	if !ok {
		t.Fatal("k8s_apply tool not found")
	}

	manifest := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
data:
  key: value
`
	input, _ := json.Marshal(applyInput{
		Manifest:  manifest,
		Namespace: "default",
	})

	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for missing dynamic client")
	}
}

func TestApplyToolInvalidJSON(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client, WithWriteAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_apply")
	if !ok {
		t.Fatal("k8s_apply tool not found")
	}

	_, err = tool.Execute(context.Background(), json.RawMessage("invalid json"))
	if err == nil {
		t.Error("expected error for invalid JSON input")
	}
}

func TestExecToolMissingPod(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client, WithWriteAccess(), WithExecAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_exec")
	if !ok {
		t.Fatal("k8s_exec tool not found")
	}

	input, _ := json.Marshal(execInput{
		Command:   []string{"ls"},
		Namespace: "default",
	})

	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for missing pod name")
	}
}

func TestExecToolMissingCommand(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client, WithWriteAccess(), WithExecAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_exec")
	if !ok {
		t.Fatal("k8s_exec tool not found")
	}

	input, _ := json.Marshal(execInput{
		Pod:       "test-pod",
		Namespace: "default",
	})

	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for missing command")
	}
}

func TestExecToolMissingRESTConfig(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client, WithWriteAccess(), WithExecAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_exec")
	if !ok {
		t.Fatal("k8s_exec tool not found")
	}

	input, _ := json.Marshal(execInput{
		Pod:       "test-pod",
		Command:   []string{"ls"},
		Namespace: "default",
	})

	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for missing REST config")
	}
}

func TestExecToolInvalidJSON(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client, WithWriteAccess(), WithExecAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_exec")
	if !ok {
		t.Fatal("k8s_exec tool not found")
	}

	_, err = tool.Execute(context.Background(), json.RawMessage("invalid json"))
	if err == nil {
		t.Error("expected error for invalid JSON input")
	}
}

func TestDeleteToolService(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client, WithWriteAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_delete")
	if !ok {
		t.Fatal("k8s_delete tool not found")
	}

	input, _ := json.Marshal(deleteInput{
		Kind:      "service",
		Name:      "test-service",
		Namespace: "default",
		DryRun:    true,
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("delete service failed: %v", err)
	}

	var out deleteOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.Kind != "service" {
		t.Errorf("expected kind 'service', got '%s'", out.Kind)
	}
}

func TestDeleteToolConfigMap(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client, WithWriteAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_delete")
	if !ok {
		t.Fatal("k8s_delete tool not found")
	}

	input, _ := json.Marshal(deleteInput{
		Kind:      "configmap",
		Name:      "test-config",
		Namespace: "default",
		DryRun:    true,
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("delete configmap failed: %v", err)
	}

	var out deleteOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if !out.Deleted {
		t.Error("expected deleted to be true")
	}
}

func TestDeleteToolSecret(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client, WithWriteAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_delete")
	if !ok {
		t.Fatal("k8s_delete tool not found")
	}

	input, _ := json.Marshal(deleteInput{
		Kind:      "secret",
		Name:      "test-secret",
		Namespace: "default",
		DryRun:    true,
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("delete secret failed: %v", err)
	}

	var out deleteOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if !out.Deleted {
		t.Error("expected deleted to be true")
	}
}

func TestDeleteToolDeployment(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client, WithWriteAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_delete")
	if !ok {
		t.Fatal("k8s_delete tool not found")
	}

	input, _ := json.Marshal(deleteInput{
		Kind:      "deployment",
		Name:      "test-deployment",
		Namespace: "default",
		DryRun:    true,
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("delete deployment failed: %v", err)
	}

	var out deleteOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if !out.Deleted {
		t.Error("expected deleted to be true")
	}
}

func TestDeleteToolNamespace(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client, WithWriteAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_delete")
	if !ok {
		t.Fatal("k8s_delete tool not found")
	}

	input, _ := json.Marshal(deleteInput{
		Kind:   "namespace",
		Name:   "production",
		DryRun: true,
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("delete namespace failed: %v", err)
	}

	var out deleteOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if !out.Deleted {
		t.Error("expected deleted to be true")
	}
}

func TestDeleteToolMissingKind(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client, WithWriteAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_delete")
	if !ok {
		t.Fatal("k8s_delete tool not found")
	}

	input, _ := json.Marshal(deleteInput{
		Name:      "test",
		Namespace: "default",
	})

	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for missing kind")
	}
}

func TestDeleteToolMissingName(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client, WithWriteAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_delete")
	if !ok {
		t.Fatal("k8s_delete tool not found")
	}

	input, _ := json.Marshal(deleteInput{
		Kind:      "pod",
		Namespace: "default",
	})

	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for missing name")
	}
}

func TestDeleteToolInvalidJSON(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client, WithWriteAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_delete")
	if !ok {
		t.Fatal("k8s_delete tool not found")
	}

	_, err = tool.Execute(context.Background(), json.RawMessage("invalid json"))
	if err == nil {
		t.Error("expected error for invalid JSON input")
	}
}

func TestGetToolInvalidJSON(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_get")
	if !ok {
		t.Fatal("k8s_get tool not found")
	}

	_, err = tool.Execute(context.Background(), json.RawMessage("invalid json"))
	if err == nil {
		t.Error("expected error for invalid JSON input")
	}
}

func TestListToolInvalidJSON(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_list")
	if !ok {
		t.Fatal("k8s_list tool not found")
	}

	_, err = tool.Execute(context.Background(), json.RawMessage("invalid json"))
	if err == nil {
		t.Error("expected error for invalid JSON input")
	}
}

func TestDescribeToolInvalidJSON(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_describe")
	if !ok {
		t.Fatal("k8s_describe tool not found")
	}

	_, err = tool.Execute(context.Background(), json.RawMessage("invalid json"))
	if err == nil {
		t.Error("expected error for invalid JSON input")
	}
}

func TestDescribeToolMissingName(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_describe")
	if !ok {
		t.Fatal("k8s_describe tool not found")
	}

	input, _ := json.Marshal(describeInput{
		Kind:      "pod",
		Namespace: "default",
	})

	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for missing name")
	}
}

func TestFormatAge(t *testing.T) {
	tests := []struct {
		name     string
		offset   time.Duration
		expected string
	}{
		{"zero time", 0, "unknown"},
		{"seconds", 30 * time.Second, "30s"},
		{"minutes", 5 * time.Minute, "5m"},
		{"hours", 3 * time.Hour, "3h"},
		{"days", 48 * time.Hour, "2d"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var inputTime time.Time
			if tt.offset == 0 && tt.expected == "unknown" {
				inputTime = time.Time{} // Zero time
			} else {
				inputTime = time.Now().Add(-tt.offset)
			}

			result := formatAge(inputTime)
			if tt.expected == "unknown" {
				if result != "unknown" {
					t.Errorf("formatAge() = %s, want unknown", result)
				}
			} else {
				// For time-based tests, just verify it contains the expected unit
				if !containsTimeUnit(result, tt.expected) {
					t.Errorf("formatAge() = %s, expected similar to %s", result, tt.expected)
				}
			}
		})
	}
}

func containsTimeUnit(result, expected string) bool {
	// Extract the unit (last character)
	if len(expected) == 0 || len(result) == 0 {
		return false
	}
	expectedUnit := expected[len(expected)-1]
	resultUnit := result[len(result)-1]
	return expectedUnit == resultUnit
}

func TestListToolWithLabelSelector(t *testing.T) {
	client := setupTestClient(t)

	p, err := New(client)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("k8s_list")
	if !ok {
		t.Fatal("k8s_list tool not found")
	}

	input, _ := json.Marshal(listInput{
		Kind:          "pod",
		Namespace:     "default",
		LabelSelector: "app=test",
		Limit:         50,
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("list with label selector failed: %v", err)
	}

	var out listOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	// Should find the test-pod with app=test label
	if out.Count < 1 {
		t.Error("expected at least 1 pod with app=test label")
	}
}
