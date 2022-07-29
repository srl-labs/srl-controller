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

// SrlinuxSpec defines the desired state of Srlinux.
type SrlinuxSpec struct {
	Config        *NodeConfig       `json:"config,omitempty"`
	NumInterfaces int               `json:"num-interfaces,omitempty"`
	Constraints   map[string]string `json:"constraints,omitempty"`
	// Model encodes SR Linux variant (ixr-d3, ixr-6e, etc)
	Model string `json:"model,omitempty"`
	// Version may be set in kne topology as a mean to explicitly provide version information
	// in case it is not encoded in the image tag
	Version string `json:"version,omitempty"`
}

// SrlinuxStatus defines the observed state of Srlinux.
type SrlinuxStatus struct {
	// Image used to run srlinux pod
	Image string `json:"image,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Srlinux is the Schema for the srlinuxes API
// +kubebuilder:printcolumn:name="Image",type="string",JSONPath=".status.image"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Srlinux struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SrlinuxSpec   `json:"spec,omitempty"`
	Status SrlinuxStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SrlinuxList contains a list of Srlinux.
type SrlinuxList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Srlinux `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Srlinux{}, &SrlinuxList{})
}

// GetConfig gets config from srlinux spec.
func (s *SrlinuxSpec) GetConfig() *NodeConfig {
	if s.Config != nil {
		return s.Config
	}

	return &NodeConfig{}
}

// GetConstraints gets constraints from srlinux spec,
// default constraints are returned if none are present in the spec.
func (s *SrlinuxSpec) GetConstraints() map[string]string {
	if s.Constraints != nil {
		return s.Constraints
	}

	return defaultConstraints
}

// GetModel gets srlinux model (aka variant) from srlinux spec,
// default srlinux variant is returned if none present in the spec.
func (s *SrlinuxSpec) GetModel() string {
	if s.Model != "" {
		return s.Model
	}

	return defaultSrlinuxVariant
}

// GetImage returns the srlinux container image name that is used in pod spec
// if Config.Image is provided it takes precedence over all other option
// if not, the Spec.Version is used as a tag for public container image ghcr.io/nokia/srlinux.
func (s *SrlinuxSpec) GetImage() string {
	img := defaultSRLinuxImageName

	if s.GetConfig().Image != "" {
		img = s.GetConfig().Image
	}

	// when image is not defined, but version is
	// the version is used as a tag for a default image repo
	if s.GetConfig().Image == "" && s.Version != "" {
		img = img + ":" + s.Version
	}

	return img
}
