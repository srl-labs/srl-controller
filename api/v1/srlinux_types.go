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
