//go:build e2e
// +build e2e

package kubernetes_workload_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"

	. "github.com/kedacore/keda/v2/tests/helper"
)

const (
	testName = "kubernetes-workload-test"
)

var (
	testNamespace           = fmt.Sprintf("%s-ns", testName)
	monitoredDeploymentName = "monitored-deployment"
	sutDeploymentName       = "sut-deployment"
	scaledObjectName        = fmt.Sprintf("%s-so", testName)
)

type templateData struct {
	TestNamespace           string
	MonitoredDeploymentName string
	SutDeploymentName       string
	ScaledObjectName        string
}

type templateValues map[string]string

const (
	monitoredDeploymentTemplate = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{.MonitoredDeploymentName}}
  namespace: {{.TestNamespace}}
  labels:
    deploy: workload-test
spec:
  replicas: 0
  selector:
    matchLabels:
      pod: workload-test
  template:
    metadata:
      labels:
        pod: workload-test
    spec:
      containers:
        - name: nginx
          image: 'nginx'`

	sutDeploymentTemplate = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{.SutDeploymentName}}
  namespace: {{.TestNamespace}}
  labels:
    deploy: workload-sut
spec:
  replicas: 0
  selector:
    matchLabels:
      pod: workload-sut
  template:
    metadata:
      labels:
        pod: workload-sut
    spec:
      containers:
      - name: nginx
        image: 'nginx'`

	scaledObjectTemplate = `apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: {{.ScaledObjectName}}
  namespace: {{.TestNamespace}}
spec:
  scaleTargetRef:
    name: {{.SutDeploymentName}}
  pollingInterval: 5
  cooldownPeriod: 5
  minReplicaCount: 0
  maxReplicaCount: 10
  advanced:
    horizontalPodAutoscalerConfig:
      behavior:
        scaleDown:
          stabilizationWindowSeconds: 5
  triggers:
  - type: kubernetes-workload
    metadata:
      podSelector: 'pod=workload-test'
      value: '1'`
)

func TestScaler(t *testing.T) {
	// setup
	t.Log("--- setting up ---")
	// Create kubernetes resources
	kc := GetKubernetesClient(t)
	data, templates := getTemplateData()

	CreateKubernetesResources(t, kc, testNamespace, data, templates)

	assert.True(t, WaitForDeploymentReplicaCount(t, kc, monitoredDeploymentName, testNamespace, 0, 60, 1),
		"replica count should be 0 after a minute")
	assert.True(t, WaitForDeploymentReplicaCount(t, kc, sutDeploymentName, testNamespace, 0, 60, 1),
		"replica count should be 0 after a minute")

	// test scaling
	testScaleUp(t, kc)
	testScaleDown(t, kc)

	// cleanup
	DeleteKubernetesResources(t, kc, testNamespace, data, templates)
}

func testScaleUp(t *testing.T, kc *kubernetes.Clientset) {
	// scale monitored deployment to 5 replicas
	KubernetesScaleDeployment(t, kc, monitoredDeploymentName, 5, testNamespace)
	assert.True(t, WaitForDeploymentReplicaCount(t, kc, sutDeploymentName, testNamespace, 5, 60, 2),
		"replica count should be 5 after a minute")

	// scale monitored deployment to 10 replicas
	KubernetesScaleDeployment(t, kc, monitoredDeploymentName, 10, testNamespace)
	assert.True(t, WaitForDeploymentReplicaCount(t, kc, sutDeploymentName, testNamespace, 10, 60, 2),
		"replica count should be 10 after a minute")
}

func testScaleDown(t *testing.T, kc *kubernetes.Clientset) {
	// scale monitored deployment to 5 replicas
	KubernetesScaleDeployment(t, kc, monitoredDeploymentName, 5, testNamespace)
	assert.True(t, WaitForDeploymentReplicaCount(t, kc, sutDeploymentName, testNamespace, 5, 60, 2),
		"replica count should be 5 after a minute")

	// scale monitored deployment to 0 replicas
	KubernetesScaleDeployment(t, kc, monitoredDeploymentName, 0, testNamespace)
	assert.True(t, WaitForDeploymentReplicaCount(t, kc, sutDeploymentName, testNamespace, 0, 60, 2),
		"replica count should be 0 after a minute")
}

func getTemplateData() (templateData, templateValues) {
	return templateData{
		TestNamespace:           testNamespace,
		MonitoredDeploymentName: monitoredDeploymentName,
		SutDeploymentName:       sutDeploymentName,
		ScaledObjectName:        scaledObjectName,
	}, templateValues{"monitoredDeploymentTemplate": monitoredDeploymentTemplate, "sutDeploymentTemplate": sutDeploymentTemplate, "scaledObjectTemplate": scaledObjectTemplate}
}
