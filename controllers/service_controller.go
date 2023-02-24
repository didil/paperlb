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
	"fmt"
	"sort"

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
)

// ServiceReconciler reconciles a Service object
type ServiceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=v1,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=v1,resources=services/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=v1,resources=services/finalizers,verbs=update
//+kubebuilder:rbac:groups=v1,resources=nodes,verbs=get;list;watch
//+kubebuilder:rbac:groups=v1,resources=nodes/status,verbs=get
//+kubebuilder:rbac:groups=lb.paperlb.com,resources=loadbalancers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=lb.paperlb.com,resources=loadbalancers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=lb.paperlb.com,resources=loadbalancerconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=lb.paperlb.com,resources=loadbalancerconfigs/status,verbs=get;update;patch

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

	var loadBalancerConfig *lbv1alpha1.LoadBalancerConfig
	if configName := svc.Annotations[loadBalancerConfigNameKey]; configName != "" {
		// find config with the name specified
		loadBalancerConfig, err = r.getLoadBalancerConfigByName(ctx, configName)
		if err != nil {
			logger.Error(err, "Failed to get config by name", "loadBalancerConfigName", configName, "error", err)
			return ctrl.Result{}, err
		}
		if loadBalancerConfig == nil {
			logger.Error(err, "Specified load balancer config not found", "loadBalancerConfigName", configName)
			return ctrl.Result{}, nil
		}
	} else {
		// find default config
		loadBalancerConfig, err = r.getDefaultLoadBalancerConfig(ctx)
		if err != nil {
			logger.Error(err, "Failed to get default config", "error", err)
			return ctrl.Result{}, err
		}
		if loadBalancerConfig == nil {
			logger.Info("Default load balancer config not found")
			return ctrl.Result{}, nil
		}
	}

	httpUpdaterURL := loadBalancerConfig.Spec.HTTPUpdaterURL
	if httpUpdaterURL == "" {
		// no http updater url set
		logger.Info("No http updater url set in load balancer config", "loadBalancerConfigName", loadBalancerConfig.Name)
		return ctrl.Result{}, nil
	}

	loadBalancerHost := loadBalancerConfig.Spec.Host
	if loadBalancerHost == "" {
		// no host set
		logger.Info("No host set in load balancer config", "loadBalancerConfigName", loadBalancerConfig.Name)
		return ctrl.Result{}, nil
	}

	if len(svc.Spec.Ports) == 0 {
		// no ports set
		logger.Info("no ports set on service")
		return ctrl.Result{}, nil
	}

	loadBalancerProtocol := svc.Spec.Ports[0].Protocol
	if loadBalancerProtocol == "" {
		// defaults to TCP
		loadBalancerProtocol = corev1.ProtocolTCP
	}
	// TCP, UDP or blank (defaults to TCP) are allowed
	if loadBalancerProtocol != corev1.ProtocolTCP && loadBalancerProtocol != corev1.ProtocolUDP {
		// protocol invalid
		logger.Info("Invalid Protocol Set for PaperLB load balancer", "loadBalancerProtocol", loadBalancerProtocol)
		return ctrl.Result{}, nil
	}

	targets, err := r.getTargets(logger, ctx, svc)
	if err != nil {
		return ctrl.Result{}, err
	}
	if targets == nil {
		// no targets, skip
		return ctrl.Result{}, err
	}

	// Define new load balancer
	lb, err := r.loadBalancerForService(svc, loadBalancerConfig.Name, httpUpdaterURL, loadBalancerHost, string(loadBalancerProtocol), targets)
	if err != nil {
		logger.Error(err, "Failed to build new load balancer", "LoadBalancer.Name", svc.Name)
		return ctrl.Result{}, err
	}

	// Check if the object already exists, if not create a new one
	existingLb := &lbv1alpha1.LoadBalancer{}
	err = r.Get(ctx, types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, existingLb)
	if err != nil {
		if apierrors.IsNotFound(err) {
			loadBalancerPort, err := r.findAvailableLoadBalancerPort(ctx, loadBalancerConfig)
			if err != nil {
				logger.Error(err, "Failed to find available load balancer port", "loadBalancerConfigName", loadBalancerConfig.Name, "error", err)
				return ctrl.Result{}, err
			}
			if loadBalancerPort == 0 {
				logger.Info("No available load balancer port found", "loadBalancerConfigName", loadBalancerConfig.Name)
				return ctrl.Result{}, nil
			}

			lb.Spec.Port = loadBalancerPort

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

	if r.lbNeedsUpdate(&logger, lb, existingLb, loadBalancerConfig) {
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

		portStatus.Port = int32(existingLb.Spec.Port)

		loadBalancerProtocol := existingLb.Spec.Protocol
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

var paperLBSystemNamespaceName = "paperlb-system"

func (r *ServiceReconciler) getLoadBalancerConfigByName(ctx context.Context, name string) (*lbv1alpha1.LoadBalancerConfig, error) {
	config := &lbv1alpha1.LoadBalancerConfig{}

	err := r.Get(ctx, types.NamespacedName{Namespace: paperLBSystemNamespaceName, Name: name}, config)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, errors.Wrapf(err, "failed to fetch load balancer config")
	}

	return config, nil
}

func (r *ServiceReconciler) lbNeedsUpdate(logger *logr.Logger, lb, existingLb *lbv1alpha1.LoadBalancer, config *lbv1alpha1.LoadBalancerConfig) bool {
	if existingLb.Spec.Port >= config.Spec.PortRange.Low && existingLb.Spec.Port <= config.Spec.PortRange.High {
		// existing port is still in valid range
		// keep the same port
		lb.Spec.Port = existingLb.Spec.Port
	} else {
		// load balancers need to be deleted manually in case of incompatible port changes on the config
		logger.Info("Load balancer config update requires port change, this case is not supported at the moment, update skipped.")
		return false
	}

	return !equality.Semantic.DeepEqual(lb.Spec, existingLb.Spec)
}

func (r *ServiceReconciler) getDefaultLoadBalancerConfig(ctx context.Context) (*lbv1alpha1.LoadBalancerConfig, error) {
	configsList := &lbv1alpha1.LoadBalancerConfigList{}
	err := r.List(ctx, configsList, client.InNamespace(paperLBSystemNamespaceName))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list configs")
	}
	if len(configsList.Items) == 0 {
		return nil, nil
	}
	// sort configs by creation date to be able to take only the first one if multiple have default set to true
	sort.Slice(configsList.Items, func(i, j int) bool {
		return configsList.Items[i].CreationTimestamp.Before(&configsList.Items[j].CreationTimestamp)
	})

	for _, config := range configsList.Items {
		if config.Spec.Default {
			return &config, nil
		}
	}

	return nil, nil
}

func (r *ServiceReconciler) findAvailableLoadBalancerPort(ctx context.Context, config *lbv1alpha1.LoadBalancerConfig) (int, error) {
	lbList := &lbv1alpha1.LoadBalancerList{}

	err := r.List(context.Background(), lbList, client.MatchingFields{loadBalancerConfigNameIndexField: string(config.Name)})
	if err != nil {
		return 0, fmt.Errorf("could not list load balancers for config name %s", config.Name)
	}
	used := map[int]bool{}
	for _, lb := range lbList.Items {
		used[lb.Spec.Port] = true
	}

	for i := config.Spec.PortRange.Low; i <= config.Spec.PortRange.High; i++ {
		if !used[i] {
			return i, nil
		}
	}

	// no available port found
	return 0, nil
}

func (r *ServiceReconciler) findExternalIP(node *corev1.Node) string {
	addrs := node.Status.Addresses
	for _, addr := range addrs {
		if addr.Type == corev1.NodeExternalIP {
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
	nodes := &corev1.NodeList{}
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

func (r *ServiceReconciler) loadBalancerForService(svc *corev1.Service, configName string, httpUpdaterURL string, loadBalancerHost string, loadBalancerProtocol string, targets []lbv1alpha1.Target) (*lbv1alpha1.LoadBalancer, error) {
	lb := &lbv1alpha1.LoadBalancer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svc.Name,
			Namespace: svc.Namespace,
		},
		Spec: lbv1alpha1.LoadBalancerSpec{
			ConfigName: configName,
			HTTPUpdater: lbv1alpha1.HTTPUpdater{
				URL: httpUpdaterURL,
			},
			Host:     loadBalancerHost,
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

const loadBalancerConfigNameKey = "lb.paperlb.com/config-name"

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

	if err := r.createLoadBalancerConfigNameIndex(mgr); err != nil {
		return errors.Wrapf(err, "failed to create load balancer config name  index")
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Service{}).
		// watches node changes to be able to update targets
		Watches(&source.Kind{Type: &corev1.Node{}}, handler.EnqueueRequestsFromMapFunc(r.mapNodeToServices)).
		// watches config changes to be able to update load balancer params
		Watches(&source.Kind{Type: &lbv1alpha1.LoadBalancerConfig{}}, handler.EnqueueRequestsFromMapFunc(r.mapLoadBalancerConfigToServices)).
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

const loadBalancerConfigNameIndexField = ".spec.ConfigName"

func (r *ServiceReconciler) createLoadBalancerConfigNameIndex(mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&lbv1alpha1.LoadBalancer{},
		loadBalancerConfigNameIndexField,
		func(object client.Object) []string {
			lb := object.(*lbv1alpha1.LoadBalancer)
			return []string{string(lb.Spec.ConfigName)}
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

func (r *ServiceReconciler) mapLoadBalancerConfigToServices(object client.Object) []reconcile.Request {
	lbConfig := object.(*lbv1alpha1.LoadBalancerConfig)

	ctx := context.Background()
	logger := log.FromContext(ctx)

	lbList := &lbv1alpha1.LoadBalancerList{}

	err := r.List(context.Background(), lbList, client.MatchingFields{loadBalancerConfigNameIndexField: string(lbConfig.Name)})
	if err != nil {
		logger.Error(err, "could not list load balancers", "lbConfigName", lbConfig.Name)
		return nil
	}

	requests := make([]reconcile.Request, 0, len(lbList.Items))

	for _, lb := range lbList.Items {
		ownerReferences := lb.GetOwnerReferences()
		for _, ownerReference := range ownerReferences {
			if ownerReference.Kind == "Service" {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: lb.Namespace,
						Name:      ownerReference.Name,
					},
				})
			}
		}
	}

	return requests
}
