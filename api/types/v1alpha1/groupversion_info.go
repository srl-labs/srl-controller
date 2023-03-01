// Copyright 2022 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

// Package v1alpha1 contains API Schema definitions for the kne v1alpha1 API group
// +kubebuilder:object:generate=true
// +groupName=kne.srlinux.dev
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	GroupName    = "kne.srlinux.dev"
	GroupVersion = "v1alpha1"
)

// nolint: gochecknoglobals
var (
	SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: GroupVersion}
	Scheme             = runtime.NewScheme()
)

func init() {
	Scheme.AddKnownTypes(SchemeGroupVersion,
		&Srlinux{},
		&SrlinuxList{},
	)
	metav1.AddToGroupVersion(Scheme, SchemeGroupVersion)
	metav1.AddMetaToScheme(Scheme)
}
