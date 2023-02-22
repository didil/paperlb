/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"strconv"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	lbv1alpha1 "github.com/didil/paperlb/api/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
)

// ServiceReconciler reconciles a Service object
type ServiceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=v1,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=v1,resources=services/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=v1,resources=services/finalizers,verbs=update
//+kubebuilder:rbac:groups=lb.paperlb.com,resources=loadbalancers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=lb.paperlb.com,resources=loadbalancers/status,verbs=get;update;patch

func (r *ServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	svc, err := r.serviceFor(ctx, req.NamespacedName)
	if err != nil {
		// Error reading the object - requeue the request.
		logger.Error(err, "Failed to get service", "error", err)
		return ctrl.Result{}, err
	}
	if svc == nil {
		logger.Info("Service resource not found. Ignoring since object must be deleted")
		return ctrl.Result{}, nil
	}

	if !svc.ObjectMeta.DeletionTimestamp.IsZero() {
		// service is being deleted, skip
		return ctrl.Result{}, nil
	}

	if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		// not a a load balancer service
		return ctrl.Result{}, nil
	}

	loadBalancerClass := svc.Spec.LoadBalancerClass
	if loadBalancerClass != nil && *loadBalancerClass != paperLBloadBalancerClass {
		// load balancer class is set and not ours
		return ctrl.Result{}, nil
	}

	httpUpdaterURL := svc.Annotations[loadBalancerHttpUpdaterURLKey]
	if httpUpdaterURL == "" {
		// no http updater url set
		logger.Info("No http updater url set for PaperLB load balancer")
		return ctrl.Result{}, nil
	}

	loadBalancerHost := svc.Annotations[loadBalancerHostKey]
	if loadBalancerHost == "" {
		// no host set
		logger.Info("No Host Set for PaperLB load balancer")
		return ctrl.Result{}, nil
	}

	loadBalancerPort := svc.Annotations[loadBalancerPortKey]
	if loadBalancerPort == "" {
		// no port set
		logger.Info("No Port Set for PaperLB load balancer")
		return ctrl.Result{}, nil
	}

	loadBalancerPortInt, err := strconv.ParseUint(loadBalancerPort, 10, 16)
	if err != nil {
		// port invalid
		logger.Info("Invalid Port Set for PaperLB load balancer", "loadBalancerPort", loadBalancerPort)
		return ctrl.Result{}, nil
	}

	loadBalancerProtocol := svc.Annotations[loadBalancerProtocolKey]
	// TCP, UDP or blank (defaults to TCP) are allowed
	if loadBalancerProtocol != string(corev1.ProtocolTCP) && loadBalancerProtocol != string(corev1.ProtocolUDP) && loadBalancerProtocol != "" {
		// protocol invalid
		logger.Info("Invalid Protocol Set for PaperLB load balancer", "loadBalancerProtocol", loadBalancerProtocol)
		return ctrl.Result{}, nil
	}

	if len(svc.Spec.Ports) == 0 {
		// no ports set
		logger.Info("no ports set on service")
		return ctrl.Result{}, nil
	}

	targets, err := r.getTargets(logger, ctx, svc)
	if err != nil {
		return ctrl.Result{}, err
	}
	if targets == nil {
		// no targets, skip
	}

	// Define new load balancer
	lb, err := r.loadBalancerForService(svc, httpUpdaterURL, loadBalancerHost, int(loadBalancerPortInt), loadBalancerProtocol, targets)
	if err != nil {
		logger.Error(err, "Failed to build new load balancer", "LoadBalancer.Name", svc.Name)
		return ctrl.Result{}, err
	}

	// Check if the object already exists, if not create a new one
	existingLb := &lbv1alpha1.LoadBalancer{}
	err = r.Get(ctx, types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, existingLb)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Creating a Load Balancer", "LoadBalancer.Name", lb.Name)
			err = r.Create(ctx, lb)
			if err != nil {
				logger.Error(err, "Failed to create Load Balancer", "LoadBalancer.Name", lb.Name)
				return ctrl.Result{}, err
			}

			// created successfully - return and requeue
			return ctrl.Result{Requeue: true}, nil
		}

		logger.Error(err, "Failed to get Load Balancer")
		return ctrl.Result{}, err
	}

	if !equality.Semantic.DeepEqual(lb.Spec, existingLb.Spec) {
		logger.Info("Updating Load Balancer", "LoadBalancer.Name", existingLb.Name)

		existingLb.Spec = lb.Spec

		err = r.Update(ctx, existingLb)
		if err != nil {
			logger.Error(err, "Failed to update Load Balancer", "LoadBalancer.Name", existingLb.Name)
			return ctrl.Result{}, err
		}

		// reset to pending
		existingLb.Status.Phase = lbv1alpha1.LoadBalancerPhasePending
		err = r.Status().Update(ctx, existingLb)
		if err != nil {
			logger.Error(err, "Failed to reset Load Balancer status to pending", "LoadBalancer.Name", existingLb.Name)
			return ctrl.Result{}, err
		}

		// Updated successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	}

	if existingLb.Status.Phase == lbv1alpha1.LoadBalancerPhaseReady {
		portStatus := corev1.PortStatus{}

		portStatus.Port = int32(loadBalancerPortInt)

		loadBalancerProtocol := svc.Annotations[loadBalancerProtocolKey]
		switch corev1.Protocol(loadBalancerProtocol) {
		case corev1.ProtocolTCP:
			portStatus.Protocol = corev1.ProtocolTCP
		case corev1.ProtocolUDP:
			portStatus.Protocol = corev1.ProtocolUDP
		default:
			portStatus.Protocol = corev1.ProtocolTCP
		}

		ports := []corev1.PortStatus{portStatus}

		targetIngresses := []corev1.LoadBalancerIngress{
			{
				IP:    loadBalancerHost,
				Ports: ports,
			},
		}

		ingresses := svc.Status.LoadBalancer.Ingress
		if !equality.Semantic.DeepEqual(targetIngresses, ingresses) {
			svc.Status.LoadBalancer.Ingress = targetIngresses

			logger.Info("Adding Load Balancer Host to service", "host", loadBalancerHost)

			err = r.Status().Update(ctx, svc)
			if err != nil {
				logger.Error(err, "Failed to update service with load balancer host", "error", err)
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *ServiceReconciler) findExternalIP(node *v1.Node) string {
	addrs := node.Status.Addresses
	for _, addr := range addrs {
		if addr.Type == v1.NodeExternalIP {
			return addr.Address
		}
	}
	return ""
}

func (r *ServiceReconciler) getTargets(logger logr.Logger, ctx context.Context, svc *corev1.Service) ([]lbv1alpha1.Target, error) {
	portsData := svc.Spec.Ports[0]

	nodePort := portsData.NodePort
	if nodePort == 0 {
		// nodeport not set
		logger.Info("nodeport not set on service")
		return nil, nil
	}

	// get nodes
	nodes := &v1.NodeList{}
	err := r.List(ctx, nodes)
	if err != nil {
		logger.Error(err, "Failed to get nodes")
		return nil, err
	}

	targets := []lbv1alpha1.Target{}
	for _, node := range nodes.Items {
		if !r.isNodeReady(&node) {
			continue
		}

		host := r.findExternalIP(&node)
		if host == "" {
			logger.Error(err, "Failed to get external ip for node. Skipping node", "node", node.Name)
			continue
		}
		targets = append(targets, lbv1alpha1.Target{Host: host, Port: int(nodePort)})
	}

	if len(targets) == 0 {
		// no targets
		logger.Info("no targets")
		return nil, nil
	}

	return targets, nil
}

func (r *ServiceReconciler) isNodeReady(node *corev1.Node) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
			// only target nodes that are healthy
			return true
		}
	}

	return false
}

func (r *ServiceReconciler) loadBalancerForService(svc *corev1.Service, httpUpdaterURL string, loadBalancerHost string, loadBalancerPortInt int, loadBalancerProtocol string, targets []lbv1alpha1.Target) (*lbv1alpha1.LoadBalancer, error) {
	lb := &lbv1alpha1.LoadBalancer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svc.Name,
			Namespace: svc.Namespace,
		},
		Spec: lbv1alpha1.LoadBalancerSpec{
			HTTPUpdater: lbv1alpha1.HTTPUpdater{
				URL: httpUpdaterURL,
			},
			Host:     loadBalancerHost,
			Port:     loadBalancerPortInt,
			Protocol: loadBalancerProtocol,
			Targets:  targets,
		},
	}
	// Set Service instance as the owner and controller
	err := ctrl.SetControllerReference(svc, lb, r.Scheme)
	if err != nil {
		return nil, err
	}

	return lb, nil
}

const loadBalancerHttpUpdaterURLKey = "lb.paperlb.com/http-updater-url"
const loadBalancerHostKey = "lb.paperlb.com/load-balancer-host"
const loadBalancerPortKey = "lb.paperlb.com/load-balancer-port"
const loadBalancerProtocolKey = "lb.paperlb.com/load-balancer-protocol"

const paperLBloadBalancerClass = "lb.paperlb.com/paperlb-class"

func (r *ServiceReconciler) serviceFor(ctx context.Context, name types.NamespacedName) (*corev1.Service, error) {
	svc := &corev1.Service{}
	err := r.Get(ctx, name, svc)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// No errors
			return nil, nil
		}

		// Error reading the object - requeue the request.
		return nil, err
	}

	return svc, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := r.createServiceTypeIndex(mgr); err != nil {
		return errors.Wrapf(err, "failed to create service type index")
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Service{}).
		Watches(&source.Kind{Type: &corev1.Node{}}, handler.EnqueueRequestsFromMapFunc(r.mapNodeToServices)).
		Owns(&lbv1alpha1.LoadBalancer{}).
		Complete(r)
}

const serviceTypeIndexField = ".spec.Type"

func (r *ServiceReconciler) createServiceTypeIndex(mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&corev1.Service{},
		serviceTypeIndexField,
		func(object client.Object) []string {
			svc := object.(*corev1.Service)
			return []string{string(svc.Spec.Type)}
		})
}

func (r *ServiceReconciler) mapNodeToServices(object client.Object) []reconcile.Request {
	node := object.(*corev1.Node)

	ctx := context.Background()
	logger := log.FromContext(ctx)

	serviceList := &corev1.ServiceList{}

	err := r.List(context.Background(), serviceList, client.MatchingFields{serviceTypeIndexField: string(corev1.ServiceTypeLoadBalancer)})
	if err != nil {
		logger.Error(err, "could not list services", "node", node.Name)
		return nil
	}

	requests := make([]reconcile.Request, 0, len(serviceList.Items))

	for _, svc := range serviceList.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(&svc),
		})
	}

	return requests

}
