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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// LoadBalancerSpec defines the desired state of LoadBalancer
type LoadBalancerSpec struct {
	// ConfigName is the loadbalancer config name
	// +kubebuilder:validation:Required
	ConfigName string `json:"configName,omitempty"`
	// HTTPUpdater is the http updater
	// +kubebuilder:validation:Required
	HTTPUpdater HTTPUpdater `json:"httpUpdater,omitempty"`
	// Host is the lb host
	// +kubebuilder:validation:Required
	Host string `json:"host,omitempty"`
	// Port is the lb host
	// +kubebuilder:validation:Required
	Port int `json:"port,omitempty"`
	// Protocol is the lb protocol
	// +kubebuilder:validation:Required
	Protocol string `json:"protocol,omitempty"`
	// Targets is the list of targets
	Targets []Target `json:"targets,omitempty"`
}

type Target struct {
	// Host is the target host
	// +kubebuilder:validation:Required
	Host string `json:"host,omitempty"`
	// Port is the target port
	// +kubebuilder:validation:Required
	Port int `json:"port,omitempty"`
}

type LoadBalancerPhase string

const (
	// LoadBalancerPhase pending
	LoadBalancerPhasePending LoadBalancerPhase = "PENDING"
	// LoadBalancerPhase ready
	LoadBalancerPhaseReady LoadBalancerPhase = "READY"
)

// LoadBalancerStatus defines the observed state of LoadBalancer
type LoadBalancerStatus struct {
	// +kubebuilder:validation:Enum=PENDING;READY
	// +kubebuilder:validation:Required
	// +kubebuilder:default=PENDING
	Phase LoadBalancerPhase `json:"phase,omitempty"`

	// TargetCount is the number of targets
	TargetCount int `json:"targetCount,omitempty"`
}

// HTTPUpdater is the http updated for the load balancer
type HTTPUpdater struct {
	// URL to request lb updates
	URL string `json:"url,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Host",type=string,JSONPath=`.spec.host`
// +kubebuilder:printcolumn:name="Port",type=string,JSONPath=`.spec.port`
// +kubebuilder:printcolumn:name="Protocol",type=string,JSONPath=`.spec.protocol`
// +kubebuilder:printcolumn:name="TargetCount",type=string,JSONPath=`.status.targetCount`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.phase`
// LoadBalancer is the Schema for the loadbalancers API
type LoadBalancer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LoadBalancerSpec   `json:"spec,omitempty"`
	Status LoadBalancerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// LoadBalancerList contains a list of LoadBalancer
type LoadBalancerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LoadBalancer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LoadBalancer{}, &LoadBalancerList{})
}
