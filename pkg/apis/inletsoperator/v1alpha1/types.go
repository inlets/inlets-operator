/*
Copyright 2017 The Kubernetes Authors.

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

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Tunnel is a specification for a Tunnel resource
// +kubebuilder:printcolumn:name="Service",type=string,JSONPath=`.spec.serviceRef.name`
// +kubebuilder:printcolumn:name="Client",priority=1,type=string,JSONPath=`.status.clientDeploymentRef.name`
// +kubebuilder:printcolumn:name="HostStatus",type=string,JSONPath=`.status.hostStatus`
// +kubebuilder:printcolumn:name="HostIP",type=string,JSONPath=`.status.hostIP`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:printcolumn:name="HostID",priority=1,type=string,JSONPath=`.status.hostId`
// +kubebuilder:printcolumn:name="UpdateServiceIP",priority=1,type=boolean,JSONPath=`.spec.updateServiceIP`
// +kubebuilder:printcolumn:name="Generated",priority=1,type=boolean,JSONPath=`.status.generated`
// +kubebuilder:subresource:status
type Tunnel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec TunnelSpec `json:"spec,omitempty"`

	// +kubebuilder:validation:Optional
	Status TunnelStatus `json:"status,omitempty"`
}

// TunnelSpec is the spec for a Tunnel resource
type TunnelSpec struct {
	// ServiceRef is the internal service to tunnel to the remote host
	ServiceRef *ResourceRef `json:"serviceRef,omitempty"`

	// +nullable
	// +kubebuilder:validation:Optional

	// AuthToken is the secret used to authenticate the tunnel client with the remote tunnel server VM
	AuthTokenRef *ResourceRef `json:"authTokenRef,omitempty"`

	// +nullable
	// +kubebuilder:validation:Optional
	// LicenseRef is the secret used to load the inlets-client
	// license, and is the same for each tunnel within the cluster
	LicenseRef *ResourceRef `json:"licenseRef,omitempty"`

	// +nullable
	// +kubebuilder:validation:Optional
	UpdateServiceIP bool `json:"updateServiceIP,omitempty"`

	// +nullable
	// +kubebuilder:validation:Optional
	// ProxyProto when set, is passed onto the tunnel server
	// in order to have it send the original source IP.
	// Note: any upstream must be able to read the Proxy Protocol header
	ProxyProto string `json:"proxyProto,omitempty"`
}

// TunnelStatus is the status for a Tunnel resource
type TunnelStatus struct {
	// Generated is set to true when the tunnel is created by the operator and false
	// when a user creates the Tunnel via YAML
	Generated bool `json:"generated,omitempty"`

	// + optional
	HostStatus string `json:"hostStatus,omitempty"`

	// + optional
	HostIP string `json:"hostIP,omitempty"`

	// + optional
	HostID string `json:"hostId,omitempty"`

	// + optional
	// +kubebuilder:validation:Optional
	AuthTokenRef *ResourceRef `json:"authTokenRef,omitempty"`

	// +nullable
	// +kubebuilder:validation:Optional
	ClientDeploymentRef *ResourceRef `json:"clientDeploymentRef,omitempty"`
}

// ResourceRef references resources across namespaces
type ResourceRef struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TunnelList is a list of Tunnel resources
type TunnelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Tunnel `json:"items"`
}
