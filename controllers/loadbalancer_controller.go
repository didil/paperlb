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
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	lbv1alpha1 "github.com/didil/paperlb/api/v1alpha1"
	"github.com/didil/paperlb/services"
)

// LoadBalancerReconciler reconciles a LoadBalancer object
type LoadBalancerReconciler struct {
	client.Client
	Scheme              *runtime.Scheme
	HTTPLBUpdaterClient services.HTTPLBUpdaterClient
}

//+kubebuilder:rbac:groups=lb.paperlb.com,resources=loadbalancers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=lb.paperlb.com,resources=loadbalancers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=lb.paperlb.com,resources=loadbalancers/finalizers,verbs=update

func (r *LoadBalancerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	lb := &lbv1alpha1.LoadBalancer{}
	err := r.Get(ctx, req.NamespacedName, lb)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			logger.Info("LoadBlancer resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}

		logger.Error(err, "Failed to get Load Balancer")
		return ctrl.Result{}, err
	}

	// examine DeletionTimestamp to determine if object is under deletion
	if lb.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent to
		// registering our finalizer.

		if !containsString(lb.ObjectMeta.Finalizers, loadBalancerFinalizerName) {
			lb.ObjectMeta.Finalizers = append(lb.ObjectMeta.Finalizers, loadBalancerFinalizerName)
			if err := r.Update(ctx, lb); err != nil {
				logger.Error(err, "Failed to update load balancer finalizers")
				return ctrl.Result{}, err
			}

			// Object updated - return and requeue
			return ctrl.Result{Requeue: true}, nil
		}
	} else {
		// The object is being deleted
		if containsString(lb.ObjectMeta.Finalizers, loadBalancerFinalizerName) {
			// our finalizer is present, delete lb via http updater
			logger.Info("Deleting load balancer via http updater")

			err := r.HTTPLBUpdaterClient.Delete(ctx, lb.Spec.HTTPUpdater.URL, &services.DeleteParams{
				BackendName: lbBackendName(lb.Namespace, lb.Name),
			})
			if err != nil {
				logger.Error(err, "Failed to delete load balancer via http updater")
				return ctrl.Result{RequeueAfter: httpUpdateRetryAfter}, err
			}

			// remove our finalizer from the list and update it.
			lb.ObjectMeta.Finalizers = removeString(lb.ObjectMeta.Finalizers, loadBalancerFinalizerName)
			if err := r.Update(context.Background(), lb); err != nil {
				logger.Error(err, "Failed to delete load balancer finalizer")
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	// check if lb update is pending
	if lb.Status.Phase != lbv1alpha1.LoadBalancerPhaseReady {
		logger.Info("Updating load balancer via http updater")

		upstreamServers := []services.UpstreamServer{}
		for _, target := range lb.Spec.Targets {
			upstreamServers = append(upstreamServers, services.UpstreamServer{Host: target.Host, Port: target.Port})
		}

		err := r.HTTPLBUpdaterClient.Update(ctx, lb.Spec.HTTPUpdater.URL, &services.UpdateParams{
			BackendName:     lbBackendName(lb.Namespace, lb.Name),
			LBPort:          lb.Spec.Port,
			LBProtocol:      lb.Spec.Protocol,
			UpstreamServers: upstreamServers,
		})
		if err != nil {
			logger.Error(err, "Failed to Update load balancer via http updater")
			return ctrl.Result{RequeueAfter: httpUpdateRetryAfter}, err
		}

		lb.Status.Phase = lbv1alpha1.LoadBalancerPhaseReady
		lb.Status.TargetCount = len(lb.Spec.Targets)
		err = r.Status().Update(ctx, lb)
		if err != nil {
			logger.Error(err, "Failed to Update load balancer status")
			return ctrl.Result{}, err
		}

		// Updated successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}

func lbBackendName(namespace, name string) string {
	return fmt.Sprintf("%s_%s", namespace, name)
}

var httpUpdateRetryAfter = 5 * time.Second

const loadBalancerFinalizerName = "lb.paperlb.com/lb-finalizer"

// SetupWithManager sets up the controller with the Manager.
func (r *LoadBalancerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&lbv1alpha1.LoadBalancer{}).
		Complete(r)
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}
