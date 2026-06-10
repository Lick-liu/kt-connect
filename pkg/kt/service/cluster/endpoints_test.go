package cluster

import (
	"context"
	"testing"

	"github.com/alibaba/kt-connect/pkg/kt/util"
	coreV1 "k8s.io/api/core/v1"
	discoveryV1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func TestKubernetes_ReconcileServiceEndpointCreatesMissingEndpoints(t *testing.T) {
	k := &Kubernetes{Clientset: testclient.NewSimpleClientset(managedService("mesh-jz2", "default", nil))}
	pod := meshPod("mesh-jz2", "default", "pod-uid", "10.244.1.134")

	if err := k.ReconcileServiceEndpoint("mesh-jz2", "default", pod, map[int]int{8080: 8080}); err != nil {
		t.Fatalf("ReconcileServiceEndpoint returned error: %v", err)
	}

	endpoints, err := k.Clientset.CoreV1().Endpoints("default").Get(context.TODO(), "mesh-jz2", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected endpoints to be created: %v", err)
	}
	if got := endpoints.Subsets[0].Addresses[0].IP; got != "10.244.1.134" {
		t.Fatalf("endpoint IP = %s, want 10.244.1.134", got)
	}
	if got := endpoints.Subsets[0].Addresses[0].TargetRef.UID; got != types.UID("pod-uid") {
		t.Fatalf("endpoint target UID = %s, want pod-uid", got)
	}
	if got := endpoints.Labels[discoveryV1.LabelSkipMirror]; got != "true" {
		t.Fatalf("endpoint skip mirror label = %q, want true", got)
	}
}

func TestKubernetes_ReconcileServiceEndpointRepairsStaleEndpointSlice(t *testing.T) {
	staleSlice := &discoveryV1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mesh-jz2-manual",
			Namespace: "default",
			Labels: map[string]string{
				util.ControlBy:               "kt",
				discoveryV1.LabelServiceName: "mesh-jz2",
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "v1",
				Kind:       "Pod",
				Name:       "mesh-jz2",
				UID:        types.UID("old-uid"),
			}},
		},
		AddressType: discoveryV1.AddressTypeIPv4,
		Endpoints: []discoveryV1.Endpoint{{
			Addresses: []string{"10.244.2.119"},
			TargetRef: &coreV1.ObjectReference{
				Kind:      "Pod",
				Name:      "mesh-jz2",
				Namespace: "default",
				UID:       types.UID("old-uid"),
			},
		}},
		Ports: []discoveryV1.EndpointPort{{
			Name: ptr("kt-8080"),
			Port: ptr[int32](8080),
		}},
	}
	k := &Kubernetes{Clientset: testclient.NewSimpleClientset(managedService("mesh-jz2", "default", nil), staleSlice)}
	pod := meshPod("mesh-jz2", "default", "new-uid", "10.244.1.134")

	if err := k.ReconcileServiceEndpoint("mesh-jz2", "default", pod, map[int]int{8080: 8080}); err != nil {
		t.Fatalf("ReconcileServiceEndpoint returned error: %v", err)
	}

	updated, err := k.Clientset.DiscoveryV1().EndpointSlices("default").Get(context.TODO(), "mesh-jz2-manual", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected endpoint slice to exist: %v", err)
	}
	if got := updated.Endpoints[0].Addresses[0]; got != "10.244.1.134" {
		t.Fatalf("endpoint slice IP = %s, want 10.244.1.134", got)
	}
	if got := updated.Endpoints[0].TargetRef.UID; got != types.UID("new-uid") {
		t.Fatalf("endpoint slice target UID = %s, want new-uid", got)
	}
	if got := updated.OwnerReferences[0].UID; got != types.UID("new-uid") {
		t.Fatalf("endpoint slice owner UID = %s, want new-uid", got)
	}
}

func TestKubernetes_ReconcileServiceEndpointSwitchesToManualEndpointsAndPrunesStaleSlices(t *testing.T) {
	staleGeneratedSlice := &discoveryV1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mesh-jz2-abcde",
			Namespace: "default",
			Labels: map[string]string{
				discoveryV1.LabelManagedBy:   "endpointslice-controller.k8s.io",
				discoveryV1.LabelServiceName: "mesh-jz2",
			},
		},
		AddressType: discoveryV1.AddressTypeIPv4,
		Endpoints: []discoveryV1.Endpoint{{
			Addresses: []string{"10.244.2.119"},
		}},
		Ports: []discoveryV1.EndpointPort{{
			Name: ptr("kt-8080"),
			Port: ptr[int32](8080),
		}},
	}
	k := &Kubernetes{Clientset: testclient.NewSimpleClientset(
		managedService("mesh-jz2", "default", map[string]string{util.KtRole: util.RoleMeshShadow}),
		staleGeneratedSlice,
	)}
	pod := meshPod("mesh-jz2", "default", "new-uid", "10.244.1.134")

	if err := k.ReconcileServiceEndpoint("mesh-jz2", "default", pod, map[int]int{8080: 8080}); err != nil {
		t.Fatalf("ReconcileServiceEndpoint returned error: %v", err)
	}

	svc, err := k.Clientset.CoreV1().Services("default").Get(context.TODO(), "mesh-jz2", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected service to exist: %v", err)
	}
	if len(svc.Spec.Selector) != 0 {
		t.Fatalf("service selector = %v, want empty selector for manual endpoints", svc.Spec.Selector)
	}
	if _, err = k.Clientset.DiscoveryV1().EndpointSlices("default").Get(context.TODO(), "mesh-jz2-abcde", metav1.GetOptions{}); err == nil {
		t.Fatalf("expected stale generated endpoint slice to be deleted")
	}
	if _, err = k.Clientset.DiscoveryV1().EndpointSlices("default").Get(context.TODO(), "mesh-jz2-manual", metav1.GetOptions{}); err != nil {
		t.Fatalf("expected manual endpoint slice to exist: %v", err)
	}
}

func TestKubernetes_RemoveServiceCleansManualEndpoints(t *testing.T) {
	endpoints := &coreV1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mesh-jz2",
			Namespace: "default",
		},
	}
	manualSlice := &discoveryV1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mesh-jz2-manual",
			Namespace: "default",
			Labels: map[string]string{
				discoveryV1.LabelServiceName: "mesh-jz2",
			},
		},
		AddressType: discoveryV1.AddressTypeIPv4,
	}
	k := &Kubernetes{Clientset: testclient.NewSimpleClientset(
		managedService("mesh-jz2", "default", nil),
		endpoints,
		manualSlice,
	)}

	if err := k.RemoveService("mesh-jz2", "default"); err != nil {
		t.Fatalf("RemoveService returned error: %v", err)
	}

	if _, err := k.Clientset.CoreV1().Endpoints("default").Get(context.TODO(), "mesh-jz2", metav1.GetOptions{}); err == nil {
		t.Fatalf("expected endpoints to be deleted")
	}
	if _, err := k.Clientset.DiscoveryV1().EndpointSlices("default").Get(context.TODO(), "mesh-jz2-manual", metav1.GetOptions{}); err == nil {
		t.Fatalf("expected endpoint slice to be deleted")
	}
}

func managedService(name, namespace string, selector map[string]string) *coreV1.Service {
	return &coreV1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				util.ControlBy: util.KubernetesToolkit,
			},
		},
		Spec: coreV1.ServiceSpec{
			Selector: selector,
		},
	}
}

func meshPod(name, namespace, uid, ip string) *coreV1.Pod {
	return &coreV1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       types.UID(uid),
		},
		Spec: coreV1.PodSpec{
			NodeName: "node-a",
		},
		Status: coreV1.PodStatus{
			PodIP: ip,
		},
	}
}

func ptr[T any](v T) *T {
	return &v
}
