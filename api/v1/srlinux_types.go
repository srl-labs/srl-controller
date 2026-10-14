/*
Copyright (c) 2021 Nokia. All rights reserved.


Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

1. Redistributions of source code must retain the above copyright notice, this
   list of conditions and the following disclaimer.

2. Redistributions in binary form must reproduce the above copyright notice,
   this list of conditions and the following disclaimer in the documentation
   and/or other materials provided with the distribution.

3. Neither the name of the copyright holder nor the names of its
   contributors may be used to endorse or promote products derived from
   this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE
FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SrlinuxSpec defines the desired state of Srlinux.
// +kubebuilder:validation:XValidation:rule="!has(self.components) || (has(self.model) && self.model in ['ixr6', 'ixr10', 'ixr6e', 'ixr10e'])",message="components are only supported on modular models ixr6, ixr10, ixr6e and ixr10e"
type SrlinuxSpec struct {
	Config        *NodeConfig       `json:"config,omitempty"`
	NumInterfaces int               `json:"num-interfaces,omitempty"`
	Constraints   map[string]string `json:"constraints,omitempty"`
	// Model encodes SR Linux variant (ixr-d3, ixr-6e, etc)
	Model string `json:"model,omitempty"`
	// Version may be set in kne topology as a mean to explicitly provide version information
	// in case it is not encoded in the image tag
	Version string `json:"version,omitempty"`
	// Components lists the line cards installed in the chassis. When set, the slot
	// configuration of the model's topology template is replaced with these components.
	// When empty, the model's topology template is used unchanged.
	// +listType=map
	// +listMapKey=slot
	Components []Component `json:"components,omitempty"`
}

// Component is a line card installed in a chassis slot.
type Component struct {
	// Slot is the chassis slot number the line card is installed in.
	// +kubebuilder:validation:Minimum=1
	Slot int32 `json:"slot"`
	// Type is the line card (IMM) model. When empty, the line card of the lowest
	// numbered slot in the model's topology template is used.
	Type IMMType `json:"type,omitempty"`
}

// IMMType is the name of an SR Linux line card (IMM) model, e.g. imm36-400g-qsfpdd.
// The enum must match the line card catalog in controllers/imm.go.
// +kubebuilder:validation:Enum=imm2-36-400g-sync-qsfpdd;imm3-18-800g-qsfpdd;imm3-36-800g-osfp;imm3-36-800g-qsfpdd;imm3-36-800g-sync-qsfpdd;imm36-400g-qsfpdd;imm60-100g-qsfp28
type IMMType string

// SrlinuxStatus defines the observed state of Srlinux.
type SrlinuxStatus struct {
	// Status is the status of the srlinux custom resource.
	// Can be one of: "created", "running", "error".
	Status string `json:"status,omitempty"`
	// Image used to run srlinux pod
	Image string `json:"image,omitempty"`
	// StartupConfig contains the status of the startup-config.
	StartupConfig StartupConfigStatus `json:"startup-config,omitempty"`
	// Ready is true if the srlinux NOS is ready to receive config.
	// This is when management server is running and initial commit is processed.
	Ready bool `json:"ready,omitempty"`
}

type StartupConfigStatus struct {
	// Phase is the phase startup-config is in. Can be one of: "pending", "loaded", "not-provided", "failed".
	Phase string `json:"phase,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Srlinux is the Schema for the srlinuxes API.
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Image",type="string",JSONPath=".status.image"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.status"
// +kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready"
// +kubebuilder:printcolumn:name="Config",type="string",JSONPath=".status.startup-config.phase"
type Srlinux struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SrlinuxSpec   `json:"spec,omitempty"`
	Status SrlinuxStatus `json:"status,omitempty"`

	// license key from license secret that contains a license file for this Srlinux
	LicenseKey string `json:"license_key,omitempty"`
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
