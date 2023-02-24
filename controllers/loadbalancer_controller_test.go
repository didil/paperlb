package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"

	lbv1alpha1 "github.com/didil/paperlb/api/v1alpha1"
	"github.com/didil/paperlb/mocks"
	"github.com/didil/paperlb/services"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("LoadBalancer controller", func() {
	const (
		namespaceName = "default"

		timeout  = time.Second * 5
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("Reconcile a LoadBalancer", func() {
		var lb *lbv1alpha1.LoadBalancer
		var gomockController *gomock.Controller
		var httpLbUpdaterClient *mocks.MockHTTPLBUpdaterClient

		BeforeEach(func() {
			gomockController = gomock.NewController(GinkgoT())

			httpLbUpdaterClient = mocks.NewMockHTTPLBUpdaterClient(gomockController)
			loadBalancerReconciler.HTTPLBUpdaterClient = httpLbUpdaterClient
		})

		It("Should update the load balancer", func() {
			defer GinkgoRecover()

			ctx := context.Background()

			loadBalancerConfigName := "my-load-balancer-config"
			loadBalancerName := "my-test-service"
			updaterURL := "http://example.com/api/v1/lb"
			lbHost := "192.168.55.99"
			lbPort := 8888
			lbProtocol := "TCP"
			nodeHost := "1.2.3.4"
			nodePort := 30100

			httpLbUpdaterClient.EXPECT().
				Update(gomock.Any(), gomock.Any(), gomock.Any()).
				Do(func(ctx context.Context, url string, params *services.UpdateParams) error {
					Expect(url).To(Equal(updaterURL))
					Expect(params.BackendName).To(Equal("default_my-test-service"))
					Expect(params.LBPort).To(Equal(lbPort))
					Expect(params.LBProtocol).To(Equal(lbProtocol))
					Expect(len(params.UpstreamServers)).To(Equal(1))
					Expect(params.UpstreamServers[0].Host).To(Equal(nodeHost))
					Expect(params.UpstreamServers[0].Port).To(Equal(nodePort))
					return nil
				}).
				AnyTimes().Return(nil)
			httpLbUpdaterClient.EXPECT().Delete(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(nil)

			lb = &lbv1alpha1.LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      loadBalancerName,
					Namespace: namespaceName,
				},
				Spec: lbv1alpha1.LoadBalancerSpec{
					ConfigName: loadBalancerConfigName,
					HTTPUpdater: lbv1alpha1.HTTPUpdater{
						URL: updaterURL,
					},
					Host:     lbHost,
					Port:     lbPort,
					Protocol: lbProtocol,
					Targets: []lbv1alpha1.Target{
						{
							Host: nodeHost,
							Port: nodePort,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, lb)).Should(Succeed())

			// wait for load balancer http update
			lb = &lbv1alpha1.LoadBalancer{}
			Eventually(func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: loadBalancerName, Namespace: namespaceName}, lb)
				if err != nil {
					return err
				}
				if lb.Status.Phase != lbv1alpha1.LoadBalancerPhaseReady {
					return fmt.Errorf("lb not ready yet %s", lb.Status.Phase)
				}
				return nil
			}, timeout, interval).Should(BeNil())

			Expect(lb.Status.Phase).To(Equal(lbv1alpha1.LoadBalancerPhaseReady))
			Expect(lb.Status.TargetCount).To(Equal(1))
		})

		AfterEach(func() {
			ctx := context.Background()
			Expect(k8sClient.Delete(ctx, lb)).Should(Succeed())
			gomockController.Finish()
		})
	})

})
