/*
Copyright 2025.

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

// KeySelector selects a key from a ConfigMap or Secret
type KeySelector struct {
	// Name of the ConfigMap or Secret
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace where the ConfigMap or Secret is located
	// If not specified, the namespace of the SecretRemix resource is used
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Key to select
	// +kubebuilder:validation:Required
	Key string `json:"key"`
}

type SecretRemixValueFrom struct {
	// +optional
	ConfigMapKeyRef *KeySelector `json:"configMapKeyRef,omitempty"`

	// +optional
	SecretKeyRef *KeySelector `json:"secretKeyRef,omitempty"`
}

type SecretRemixDataFrom struct {
	// +kubebuilder:validation:Required
	Key string `json:"key"`

	// +optional
	Value string `json:"value,omitempty"`

	// +optional
	ValueFrom *SecretRemixValueFrom `json:"valueFrom,omitempty"`
}

// SecretRemixStatus defines the observed state of SecretRemix.
type SecretRemixStatus struct {
	// ManagedSecret is the name of the secret that is managed by this SecretRemix
	// +optional
	ManagedSecret string `json:"managedSecret,omitempty"`

	// WatchedResources lists the ConfigMaps and Secrets being watched
	// +optional
	WatchedResources []WatchedResource `json:"watchedResources,omitempty"`

	// LastSyncTime is the last time the secret was synced
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// Conditions represent the latest available observations of an object's state
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// WatchedResource identifies a resource that is being watched for changes
type WatchedResource struct {
	// Type is the type of resource (ConfigMap or Secret)
	Type string `json:"type"`

	// Name is the name of the resource
	Name string `json:"name"`

	// Namespace is the namespace of the resource
	Namespace string `json:"namespace"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// SecretRemix is the Schema for the secretremixes API.
type SecretRemix struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	DataFrom []SecretRemixDataFrom `json:"dataFrom,omitempty"`
	Status   SecretRemixStatus     `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SecretRemixList contains a list of SecretRemix.
type SecretRemixList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SecretRemix `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SecretRemix{}, &SecretRemixList{})
}
