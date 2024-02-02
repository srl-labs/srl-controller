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
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// key value for a combined license file stored in a secret.
const allSecretKey = "all.key"

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

// GetImageVersion finds an srlinux image version by looking at the Image field of the spec
// as well as at Version field.
// When Version field is set it is returned.
// In other cases, Image string is evaluated and it's tag substring is parsed.
// If no tag is present, or tag is latest, the 0.0 version is assumed to be in use.
func (s *SrlinuxSpec) GetImageVersion() *SrlVersion {
	if s.Version != "" {
		return parseVersionString(s.Version)
	}

	var tag string

	split := strings.Split(s.GetImage(), ":")
	if len(split) == 2 { //nolint:gomnd
		tag = split[1]
	}

	return parseVersionString(tag)
}

// InitLicenseKey sets the Srlinux.LicenseKey to a value of a key
// that matches MAJOR-MINOR.key of a passed secret.
// Where MAJOR-MINOR is retrieved from the image version.
// If such key doesn't exist, it checks if a wildcard `all.key` is found in that secret,
// if nothing found, LicenseKey stays empty, which denotes that no license was found for Srlinux.
func (s *Srlinux) InitLicenseKey(
	_ context.Context,
	secret *corev1.Secret,
	version *SrlVersion,
) {
	if secret == nil {
		return
	}

	versionSecretKey := fmt.Sprintf("%s-%s.key", version.Major, version.Minor)
	if _, ok := secret.Data[versionSecretKey]; ok {
		s.LicenseKey = versionSecretKey

		return
	}

	if _, ok := secret.Data[allSecretKey]; ok {
		s.LicenseKey = allSecretKey

		return
	}
}
