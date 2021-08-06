/*
Copyright 2021.

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

// SrlinuxSpec defines the desired state of Srlinux
type SrlinuxSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Config *NodeConfig `json:"config,omitempty"`
}

// SrlinuxStatus defines the observed state of Srlinux
type SrlinuxStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Srlinux is the Schema for the srlinuxes API
type Srlinux struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SrlinuxSpec   `json:"spec,omitempty"`
	Status SrlinuxStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SrlinuxList contains a list of Srlinux
type SrlinuxList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Srlinux `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Srlinux{}, &SrlinuxList{})
}
