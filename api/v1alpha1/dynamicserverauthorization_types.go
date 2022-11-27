/*
Copyright 2022 Ripta Pasay.

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
	serverauthorizationv1beta1 "github.com/linkerd/linkerd2/controller/gen/apis/serverauthorization/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DynamicServerAuthorizationSpec defines the desired state of DynamicServerAuthorization
type DynamicServerAuthorizationSpec struct {
	CommonMetadata CommonMeta `json:"commonMetadata,omitempty"`

	Server serverauthorizationv1beta1.Server `json:"server,omitempty"`
	Client Client                            `json:"client,omitempty"`
}

type CommonMeta struct {
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// Client describes ways that clients may be authorized to access a server.
type Client struct {
	// Healthcheck authorizes node IP addresses. This is useful for clusters in which
	// the kubelet IP addresses are not predictable.
	Healthcheck bool `json:"healthcheck,omitempty"`
	// MeshTLS is used to dynamically authorize clients to a server.
	MeshTLS *MeshTLS `json:"meshTLS,omitempty"`
}

type MeshTLS struct {
	// ServiceAccounts defines the authorized service accounts. At most 16 items
	// may be specified in a single dynamic server authorization.
	//
	//+kubebuilder:validation:MaxItems=16
	ServiceAccounts []*ServiceAccountSelector `json:"serviceAccounts,omitempty"`
}

type ServiceAccountSelector struct {
	// Name defines the name of the service account.
	Name              string                `json:"name,omitempty"`
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`
}

// DynamicServerAuthorizationStatus defines the observed state of DynamicServerAuthorization
type DynamicServerAuthorizationStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// DynamicServerAuthorization is the Schema for the dynamicserverauthorizations API
type DynamicServerAuthorization struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DynamicServerAuthorizationSpec   `json:"spec,omitempty"`
	Status DynamicServerAuthorizationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DynamicServerAuthorizationList contains a list of DynamicServerAuthorization
type DynamicServerAuthorizationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DynamicServerAuthorization `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DynamicServerAuthorization{}, &DynamicServerAuthorizationList{})
}
