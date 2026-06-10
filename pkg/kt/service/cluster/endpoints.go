package cluster

import (
	"context"
	"fmt"
	"sort"

	"github.com/alibaba/kt-connect/pkg/kt/util"
	coreV1 "k8s.io/api/core/v1"
	discoveryV1 "k8s.io/api/discovery/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const endpointSliceManagedBy = "kt-connect"

// ReconcileServiceEndpoint makes kt-created services route to the current pod IP.
// It intentionally owns a deterministic EndpointSlice to repair stale or missing
// endpoint state when the Kubernetes endpoint controller lags or misses updates.
func (k *Kubernetes) ReconcileServiceEndpoint(serviceName, namespace string, pod *coreV1.Pod, ports map[int]int) error {
	if pod == nil {
		return fmt.Errorf("pod is nil")
	}
	if pod.Status.PodIP == "" {
		return fmt.Errorf("pod %s has no pod IP", pod.Name)
	}
	if len(ports) == 0 {
		return fmt.Errorf("no endpoint ports specified for service %s", serviceName)
	}
	if err := k.useManualServiceEndpoints(serviceName, namespace); err != nil {
		return err
	}
	if err := k.reconcileEndpoints(serviceName, namespace, pod, ports); err != nil {
		return err
	}
	if err := k.reconcileEndpointSlice(serviceName, namespace, pod, ports); err != nil {
		return err
	}
	return k.removeStaleEndpointSlices(serviceName, namespace)
}

func (k *Kubernetes) useManualServiceEndpoints(serviceName, namespace string) error {
	svc, err := k.Clientset.CoreV1().Services(namespace).Get(context.TODO(), serviceName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if svc.Labels[util.ControlBy] != util.KubernetesToolkit {
		return fmt.Errorf("service %s is not managed by kt-connect", serviceName)
	}
	if len(svc.Spec.Selector) == 0 {
		return nil
	}
	svc.Spec.Selector = nil
	_, err = k.Clientset.CoreV1().Services(namespace).Update(context.TODO(), svc, metav1.UpdateOptions{})
	return err
}

func (k *Kubernetes) reconcileEndpoints(serviceName, namespace string, pod *coreV1.Pod, ports map[int]int) error {
	desired := &coreV1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: namespace,
			Labels: map[string]string{
				util.ControlBy:              util.KubernetesToolkit,
				discoveryV1.LabelSkipMirror: "true",
			},
			Annotations: runtimeAnnotations(map[string]string{}),
		},
		Subsets: []coreV1.EndpointSubset{{
			Addresses: []coreV1.EndpointAddress{{
				IP:       pod.Status.PodIP,
				NodeName: &pod.Spec.NodeName,
				TargetRef: &coreV1.ObjectReference{
					Kind:      "Pod",
					Name:      pod.Name,
					Namespace: namespace,
					UID:       pod.UID,
				},
			}},
			Ports: endpointPorts(ports),
		}},
	}

	current, err := k.Clientset.CoreV1().Endpoints(namespace).Get(context.TODO(), serviceName, metav1.GetOptions{})
	if k8sErrors.IsNotFound(err) {
		_, err = k.Clientset.CoreV1().Endpoints(namespace).Create(context.TODO(), desired, metav1.CreateOptions{})
		return err
	}
	if err != nil {
		return err
	}
	desired.ResourceVersion = current.ResourceVersion
	_, err = k.Clientset.CoreV1().Endpoints(namespace).Update(context.TODO(), desired, metav1.UpdateOptions{})
	return err
}

func (k *Kubernetes) reconcileEndpointSlice(serviceName, namespace string, pod *coreV1.Pod, ports map[int]int) error {
	sliceName := manualEndpointSliceName(serviceName)
	desired := &discoveryV1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sliceName,
			Namespace: namespace,
			Labels: map[string]string{
				util.ControlBy:               util.KubernetesToolkit,
				discoveryV1.LabelManagedBy:   endpointSliceManagedBy,
				discoveryV1.LabelServiceName: serviceName,
			},
			Annotations: runtimeAnnotations(map[string]string{}),
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "v1",
				Kind:       "Pod",
				Name:       pod.Name,
				UID:        pod.UID,
			}},
		},
		AddressType: discoveryV1.AddressTypeIPv4,
		Endpoints: []discoveryV1.Endpoint{{
			Addresses: []string{pod.Status.PodIP},
			Conditions: discoveryV1.EndpointConditions{
				Ready:       boolPtr(true),
				Serving:     boolPtr(true),
				Terminating: boolPtr(false),
			},
			NodeName: &pod.Spec.NodeName,
			TargetRef: &coreV1.ObjectReference{
				Kind:      "Pod",
				Name:      pod.Name,
				Namespace: namespace,
				UID:       pod.UID,
			},
		}},
		Ports: endpointSlicePorts(ports),
	}

	current, err := k.Clientset.DiscoveryV1().EndpointSlices(namespace).Get(context.TODO(), sliceName, metav1.GetOptions{})
	if k8sErrors.IsNotFound(err) {
		_, err = k.Clientset.DiscoveryV1().EndpointSlices(namespace).Create(context.TODO(), desired, metav1.CreateOptions{})
		return err
	}
	if err != nil {
		return err
	}
	desired.ResourceVersion = current.ResourceVersion
	_, err = k.Clientset.DiscoveryV1().EndpointSlices(namespace).Update(context.TODO(), desired, metav1.UpdateOptions{})
	return err
}

func (k *Kubernetes) removeStaleEndpointSlices(serviceName, namespace string) error {
	slices, err := k.Clientset.DiscoveryV1().EndpointSlices(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", discoveryV1.LabelServiceName, serviceName),
	})
	if err != nil {
		return err
	}
	manualName := manualEndpointSliceName(serviceName)
	for _, slice := range slices.Items {
		if slice.Name == manualName {
			continue
		}
		err = k.Clientset.DiscoveryV1().EndpointSlices(namespace).Delete(context.TODO(), slice.Name, metav1.DeleteOptions{})
		if err != nil && !k8sErrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (k *Kubernetes) cleanupManualServiceEndpoints(serviceName, namespace string) error {
	err := k.Clientset.CoreV1().Endpoints(namespace).Delete(context.TODO(), serviceName, metav1.DeleteOptions{})
	if err != nil && !k8sErrors.IsNotFound(err) {
		return err
	}
	slices, err := k.Clientset.DiscoveryV1().EndpointSlices(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", discoveryV1.LabelServiceName, serviceName),
	})
	if err != nil {
		return err
	}
	for _, slice := range slices.Items {
		err = k.Clientset.DiscoveryV1().EndpointSlices(namespace).Delete(context.TODO(), slice.Name, metav1.DeleteOptions{})
		if err != nil && !k8sErrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func endpointPorts(ports map[int]int) []coreV1.EndpointPort {
	keys := sortedPortKeys(ports)
	endpointPorts := make([]coreV1.EndpointPort, 0, len(keys))
	for _, servicePort := range keys {
		endpointPorts = append(endpointPorts, coreV1.EndpointPort{
			Name:     fmt.Sprintf("kt-%d", servicePort),
			Port:     int32(ports[servicePort]),
			Protocol: coreV1.ProtocolTCP,
		})
	}
	return endpointPorts
}

func endpointSlicePorts(ports map[int]int) []discoveryV1.EndpointPort {
	keys := sortedPortKeys(ports)
	endpointPorts := make([]discoveryV1.EndpointPort, 0, len(keys))
	protocol := coreV1.ProtocolTCP
	for _, servicePort := range keys {
		endpointPorts = append(endpointPorts, discoveryV1.EndpointPort{
			Name:     stringPtr(fmt.Sprintf("kt-%d", servicePort)),
			Port:     int32Ptr(int32(ports[servicePort])),
			Protocol: &protocol,
		})
	}
	return endpointPorts
}

func sortedPortKeys(ports map[int]int) []int {
	keys := make([]int, 0, len(ports))
	for servicePort := range ports {
		keys = append(keys, servicePort)
	}
	sort.Ints(keys)
	return keys
}

func manualEndpointSliceName(serviceName string) string {
	return serviceName + "-manual"
}

func stringPtr(value string) *string {
	return &value
}

func int32Ptr(value int32) *int32 {
	return &value
}

func boolPtr(value bool) *bool {
	return &value
}
