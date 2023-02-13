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
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	lbv1alpha1 "github.com/didil/paperlb/api/v1alpha1"
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

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Service object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *ServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger = logger.WithValues("service", req.NamespacedName)

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

	if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		// not a a load balancer service
		return ctrl.Result{}, nil
	}

	logger.Info("Reconciling service", "type", svc.Spec.Type)
	time.Sleep(5 * time.Second)

	loadBalancerClass := svc.Spec.LoadBalancerClass
	if loadBalancerClass != nil && *loadBalancerClass != paperLBloadBalancerClass {
		// load balancer class is set and not ours
		return ctrl.Result{}, nil
	}

	loadBalancerIP := svc.Annotations[loadBalancerIPKey]
	if loadBalancerIP == "" {
		// no ip set
		return ctrl.Result{}, nil
	}

	ingresses := svc.Status.LoadBalancer.Ingress

	targetIngresses := []corev1.LoadBalancerIngress{
		{IP: loadBalancerIP},
	}

	if equality.Semantic.DeepEqual(targetIngresses, ingresses) {
		// nothing to do
		return ctrl.Result{}, nil
	}

	svc.Status.LoadBalancer.Ingress = targetIngresses

	logger.Info("Adding Load Balancer IP to service", "ip", loadBalancerIP)

	err = r.Status().Update(ctx, svc)
	if err != nil {
		logger.Error(err, "Failed to update service with load balancer ip", "error", err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

const loadBalancerIPKey = "lb.paperlb.com/load-balancer-ip"
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
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Service{}).
		Owns(&lbv1alpha1.LoadBalancer{}).
		Complete(r)
}
