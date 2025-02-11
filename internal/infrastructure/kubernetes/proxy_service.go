// Copyright Envoy Gateway Authors
// SPDX-License-Identifier: Apache-2.0
// The full text of the Apache license is available in the LICENSE file at
// the root of the repo.

package kubernetes

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/envoyproxy/gateway/internal/gatewayapi"
	"github.com/envoyproxy/gateway/internal/ir"
)

// expectedProxyService returns the expected Service based on the provided infra.
func (i *Infra) expectedProxyService(infra *ir.Infra) (*corev1.Service, error) {
	var ports []corev1.ServicePort
	for _, listener := range infra.Proxy.Listeners {
		for _, port := range listener.Ports {
			target := intstr.IntOrString{IntVal: port.ContainerPort}
			protocol := corev1.ProtocolTCP
			if port.Protocol == ir.UDPProtocolType {
				protocol = corev1.ProtocolUDP
			}
			p := corev1.ServicePort{
				Name:       port.Name,
				Protocol:   protocol,
				Port:       port.ServicePort,
				TargetPort: target,
			}
			ports = append(ports, p)
		}
	}

	// Set the labels based on the owning gatewayclass name.
	labels := envoyLabels(infra.GetProxyInfra().GetProxyMetadata().Labels)
	if len(labels[gatewayapi.OwningGatewayNamespaceLabel]) == 0 || len(labels[gatewayapi.OwningGatewayNameLabel]) == 0 {
		return nil, fmt.Errorf("missing owning gateway labels")
	}

	// Get annotations
	var annotations map[string]string
	provider := infra.GetProxyInfra().GetProxyConfig().GetProvider()
	svcCfg := provider.GetKubeResourceProvider().EnvoyService
	if svcCfg != nil {
		annotations = svcCfg.Annotations

	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   i.Namespace,
			Name:        expectedResourceHashedName(infra.Proxy.Name),
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			Type:            corev1.ServiceTypeLoadBalancer,
			Ports:           ports,
			Selector:        getSelector(labels).MatchLabels,
			SessionAffinity: corev1.ServiceAffinityNone,
			// Preserve the client source IP and avoid a second hop for LoadBalancer.
			ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyTypeLocal,
		},
	}

	return svc, nil
}

// createOrUpdateProxyService creates a Service in the kube api server based on the provided infra,
// if it doesn't exist or updates it if it does.
func (i *Infra) createOrUpdateProxyService(ctx context.Context, infra *ir.Infra) error {
	svc, err := i.expectedProxyService(infra)
	if err != nil {
		return fmt.Errorf("failed to generate expected service: %w", err)
	}
	return i.createOrUpdateService(ctx, svc)
}

// deleteProxyService deletes the Envoy Service in the kube api server, if it exists.
func (i *Infra) deleteProxyService(ctx context.Context, infra *ir.Infra) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: i.Namespace,
			Name:      expectedResourceHashedName(infra.Proxy.Name),
		},
	}

	return i.deleteService(ctx, svc)
}
