package controllers

import (
	"context"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	lbv1alpha1 "github.com/didil/paperlb/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Service controller", func() {
	const (
		namespaceName = "default"

		timeout  = time.Second * 5
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("When creating a service", func() {
		var service *corev1.Service
		var loadBalancer *lbv1alpha1.LoadBalancer
		var node *corev1.Node

		It("Should create the load blancer crd", func() {
			ctx := context.Background()

			serviceName := "test-service"
			loadBalancerName := "test-service"
			updaterURL := "http://example.com/api/v1/lb"
			lbHost := "192.168.55.99"
			lbPort := 8888
			lbProtocol := "TCP"

			port := 8000
			targetPort := 8100
			nodePort := 30100

			nodeHost := "1.2.3.4"

			service = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      serviceName,
					Namespace: namespaceName,
					Annotations: map[string]string{
						"lb.paperlb.com/http-updater-url":       updaterURL,
						"lb.paperlb.com/load-balancer-host":     lbHost,
						"lb.paperlb.com/load-balancer-port":     strconv.Itoa(lbPort),
						"lb.paperlb.com/load-balancer-protocol": lbProtocol,
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Port:       int32(port),
							TargetPort: intstr.FromInt(targetPort),
							NodePort:   int32(nodePort),
						},
					},
					Type: corev1.ServiceTypeLoadBalancer,
				},
			}
			Expect(k8sClient.Create(ctx, service)).Should(Succeed())

			node = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: serviceName,
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{
						{
							Type:    corev1.NodeExternalIP,
							Address: nodeHost,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, node)).Should(Succeed())

			// wait for load balancer creation
			Eventually(func() error {
				loadBalancer = &lbv1alpha1.LoadBalancer{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: loadBalancerName, Namespace: namespaceName}, loadBalancer)
				if err != nil {
					return err
				}

				Expect(loadBalancer.OwnerReferences).To(HaveLen(1))
				Expect(loadBalancer.OwnerReferences[0].UID).To(Equal(service.UID))

				Expect(loadBalancer.Spec.HTTPUpdater.URL).To(Equal(updaterURL))
				Expect(loadBalancer.Spec.Host).To(Equal(lbHost))
				Expect(loadBalancer.Spec.Port).To(Equal(lbPort))
				Expect(loadBalancer.Spec.Protocol).To(Equal(lbProtocol))
				Expect(loadBalancer.Spec.Targets).To(HaveLen(1))
				Expect(loadBalancer.Spec.Targets[0]).To(Equal(lbv1alpha1.Target{
					Host: nodeHost,
					Port: nodePort,
				}))

				return nil
			}, timeout, interval).Should(BeNil())

		})

		AfterEach(func() {
			ctx := context.Background()
			Expect(k8sClient.Delete(ctx, node)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, loadBalancer)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, service)).Should(Succeed())
		})
	})

})
