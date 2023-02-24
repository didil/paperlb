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

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// LoadBalancerConfigSpec defines the desired state of LoadBalancerConfig
type LoadBalancerConfigSpec struct {
	// Default defines if this config is the default config
	// +kubebuilder:validation:Required
	Default bool `json:"default,omitempty"`
	// HTTPUpdaterURL is the http updater url
	// +kubebuilder:validation:Required
	HTTPUpdaterURL string `json:"httpUpdaterURL,omitempty"`
	// Host is the load balancer host
	// +kubebuilder:validation:Required
	Host string `json:"host,omitempty"`
	// PortRange is the load balancer port range
	// +kubebuilder:validation:Required
	PortRange PortRange `json:"portRange,omitempty"`
}

// PortRange defines the load balancer port range
type PortRange struct {
	// Low is the lower limit of the port range
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	Low int `json:"low,omitempty"`
	// High is the higher limit of the port range
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Maximum=65535
	High int `json:"high,omitempty"`
}

// LoadBalancerConfigStatus defines the observed state of LoadBalancerConfig
type LoadBalancerConfigStatus struct {
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// LoadBalancerConfig is the Schema for the loadbalancerconfigs API
type LoadBalancerConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LoadBalancerConfigSpec   `json:"spec,omitempty"`
	Status LoadBalancerConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// LoadBalancerConfigList contains a list of LoadBalancerConfig
type LoadBalancerConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LoadBalancerConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LoadBalancerConfig{}, &LoadBalancerConfigList{})
}
