/*
Copyright 2024 welasco.

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

// ConfigSpec defines the desired state of Config
type ConfigSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	AzKeyVaultURL string `json:"azKeyVaultURL"`

	// +kubebuilder:validation:Optional
	AzKeyVaultClientID string `json:"azKeyvaultClientId"`

	// +kubebuilder:validation:Optional
	AzKeyVaultClientSecret string `json:"azKeyVaultClientSecret"`

	// +kubebuilder:validation:Optional
	AzKeyVaultTenantID string `json:"azKeyVaultTenantId"`

	// +kubebuilder:validation:Optional
	FilterMatchingLabels map[string]string `json:"filterMatchingLabels"`

	// +kubebuilder:validation:Optional
	FilterMatchingAnnotations map[string]string `json:"filterMatchingAnnotations"`

	// +kubebuilder:validation:Optional
	FilterMatchingNamespace []string `json:"filterMatchingNamespace"`

	// +kubebuilder:default:=true
	AllowAzKeyVaultCertificateDeletion bool `json:"allowAzKeyVaultCertificateDeletion"`
}

// ConfigStatus defines the observed state of Config
type ConfigStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	ConfigStatus        string `json:"syncStatus"`
	ConfigStatusMessage string `json:"syncStatusMessage"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Config is the Schema for the configs API
type Config struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConfigSpec   `json:"spec,omitempty"`
	Status ConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ConfigList contains a list of Config
type ConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Config `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Config{}, &ConfigList{})
}
