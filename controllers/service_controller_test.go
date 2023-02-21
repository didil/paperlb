package controllers

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	lbv1alpha1 "github.com/didil/paperlb/api/v1alpha1"
	"github.com/didil/paperlb/mocks"
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

	Context("Reconcile a service", func() {
		var gomockController *gomock.Controller
		var httpLbUpdaterClient *mocks.MockHTTPLBUpdaterClient
		var service *corev1.Service
		var loadBalancer *lbv1alpha1.LoadBalancer
		var node1 *corev1.Node
		var node2 *corev1.Node

		BeforeEach(func() {
			gomockController = gomock.NewController(GinkgoT())

			httpLbUpdaterClient = mocks.NewMockHTTPLBUpdaterClient(gomockController)
			loadBalancerReconciler.HTTPLBUpdaterClient = httpLbUpdaterClient
		})

		It("Should create/update the load balancer", func() {
			httpLbUpdaterClient.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(nil)
			httpLbUpdaterClient.EXPECT().Delete(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(nil)
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

			nodeHost1 := "1.2.3.4"
			node1 = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-1",
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{
						{
							Type:    corev1.NodeExternalIP,
							Address: nodeHost1,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, node1)).Should(Succeed())

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

			// wait for load balancer creation
			loadBalancer = &lbv1alpha1.LoadBalancer{}
			Eventually(func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: loadBalancerName, Namespace: namespaceName}, loadBalancer)

				return err
			}, timeout, interval).Should(BeNil())

			Expect(loadBalancer.OwnerReferences).To(HaveLen(1))
			Expect(loadBalancer.OwnerReferences[0].UID).To(Equal(service.UID))

			Expect(loadBalancer.Spec.HTTPUpdater.URL).To(Equal(updaterURL))
			Expect(loadBalancer.Spec.Host).To(Equal(lbHost))
			Expect(loadBalancer.Spec.Port).To(Equal(lbPort))
			Expect(loadBalancer.Spec.Protocol).To(Equal(lbProtocol))
			Expect(loadBalancer.Spec.Targets).To(HaveLen(1))
			Expect(loadBalancer.Spec.Targets[0]).To(Equal(lbv1alpha1.Target{
				Host: nodeHost1,
				Port: nodePort,
			}))

			nodeHost2 := "1.2.3.5"
			node2 = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-2",
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{
						{
							Type:    corev1.NodeExternalIP,
							Address: nodeHost2,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, node2)).Should(Succeed())

			// wait for load balancer update
			loadBalancer = &lbv1alpha1.LoadBalancer{}
			Eventually(func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: loadBalancerName, Namespace: namespaceName}, loadBalancer)
				if err != nil {
					return err
				}

				if len(loadBalancer.Spec.Targets) != 2 {
					return fmt.Errorf("there should be 2 targets")
				}

				return nil
			}, timeout, interval).Should(BeNil())

			Expect(loadBalancer.OwnerReferences).To(HaveLen(1))
			Expect(loadBalancer.OwnerReferences[0].UID).To(Equal(service.UID))

			Expect(loadBalancer.Spec.HTTPUpdater.URL).To(Equal(updaterURL))
			Expect(loadBalancer.Spec.Host).To(Equal(lbHost))
			Expect(loadBalancer.Spec.Port).To(Equal(lbPort))
			Expect(loadBalancer.Spec.Protocol).To(Equal(lbProtocol))
			Expect(loadBalancer.Spec.Targets).To(ContainElements(
				lbv1alpha1.Target{
					Host: nodeHost1,
					Port: nodePort,
				}, lbv1alpha1.Target{
					Host: nodeHost2,
					Port: nodePort,
				},
			))

		})

		AfterEach(func() {
			ctx := context.Background()
			Expect(k8sClient.Delete(ctx, node1)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, node2)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, loadBalancer)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, service)).Should(Succeed())
			gomockController.Finish()
		})
	})

})
