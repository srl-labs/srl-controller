// Copyright 2022 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

// Package v1alpha1 is an v1alpha version of a Clientset for SR Linux customer resource.
package v1alpha1

// note to my future self: see https://www.martin-helmich.de/en/blog/kubernetes-crd-client.html for details

import (
	"context"
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	srlinuxv1 "github.com/srl-labs/srl-controller/api/types/v1alpha1"
)

// ErrUpdateFailed occurs when update operation fails on srlinux CR.
var ErrUpdateFailed = errors.New("operation update failed")

// SrlinuxInterface provides access to the Srlinux CRD.
type SrlinuxInterface interface {
	List(ctx context.Context, opts metav1.ListOptions) (*srlinuxv1.SrlinuxList, error)
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*srlinuxv1.Srlinux, error)
	Create(ctx context.Context, srlinux *srlinuxv1.Srlinux, opts metav1.CreateOptions) (*srlinuxv1.Srlinux, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	Unstructured(ctx context.Context, name string, opts metav1.GetOptions, subresources ...string) (*unstructured.Unstructured, error)
	Update(ctx context.Context, obj *unstructured.Unstructured, opts metav1.UpdateOptions) (*srlinuxv1.Srlinux, error)
}

// Interface is the clientset interface for srlinux.
type Interface interface {
	Srlinux(namespace string) SrlinuxInterface
}

// Clientset is a client for the srlinux crds.
type Clientset struct {
	dInterface dynamic.NamespaceableResourceInterface
}

var gvr = schema.GroupVersionResource{
	Group:    srlinuxv1.GroupName,
	Version:  srlinuxv1.GroupVersion,
	Resource: "srlinuxs",
}

func GVR() schema.GroupVersionResource {
	return gvr
}

var groupVersion = &schema.GroupVersion{
	Group:   srlinuxv1.GroupName,
	Version: srlinuxv1.GroupVersion,
}

func GV() *schema.GroupVersion {
	return groupVersion
}

// NewForConfig returns a new Clientset based on c.
func NewForConfig(c *rest.Config) (*Clientset, error) {
	config := *c
	config.ContentConfig.GroupVersion = &schema.GroupVersion{Group: GVR().Group, Version: GVR().Version}
	config.APIPath = "/apis"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	config.UserAgent = rest.DefaultKubernetesUserAgent()

	dClient, err := dynamic.NewForConfig(c)
	if err != nil {
		return nil, err
	}

	dInterface := dClient.Resource(gvr)

	return &Clientset{dInterface: dInterface}, nil
}

// Srlinux initializes srlinuxClient struct which implements SrlinuxInterface.
func (c *Clientset) Srlinux(namespace string) SrlinuxInterface {
	return &srlinuxClient{
		dInterface: c.dInterface,
		ns:         namespace,
	}
}

type srlinuxClient struct {
	dInterface dynamic.NamespaceableResourceInterface
	ns         string
}

// List gets a list of SRLinux resources.
func (s *srlinuxClient) List(ctx context.Context, opts metav1.ListOptions) (*srlinuxv1.SrlinuxList, error) {
	u, err := s.dInterface.Namespace(s.ns).List(ctx, opts)
	if err != nil {
		return nil, err
	}
	result := srlinuxv1.SrlinuxList{}
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), &result); err != nil {
		return nil, fmt.Errorf("failed to type assert return to SrlinuxList: %w", err)
	}
	return &result, nil
}

// Get gets SRLinux resource.
func (s *srlinuxClient) Get(ctx context.Context, name string, opts metav1.GetOptions) (*srlinuxv1.Srlinux, error) {
	u, err := s.dInterface.Namespace(s.ns).Get(ctx, name, opts)
	if err != nil {
		return nil, err
	}
	result := srlinuxv1.Srlinux{}
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), &result); err != nil {
		return nil, fmt.Errorf("failed to type assert return to Srlinux: %w", err)
	}
	return &result, nil
}

// Create creates SRLinux resource.
func (s *srlinuxClient) Create(ctx context.Context, srlinux *srlinuxv1.Srlinux, opts metav1.CreateOptions) (*srlinuxv1.Srlinux, error) {
	gvk, err := apiutil.GVKForObject(srlinux, srlinuxv1.Scheme)
	if err != nil {
		return nil, fmt.Errorf("failed to get gvk for Srlinux: %w", err)
	}
	srlinux.TypeMeta = metav1.TypeMeta{
		Kind:       gvk.Kind,
		APIVersion: gvk.GroupVersion().String(),
	}
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(srlinux)
	if err != nil {
		return nil, fmt.Errorf("failed to convert Srlinux to unstructured: %w", err)
	}
	u, err := s.dInterface.Namespace(s.ns).Create(ctx, &unstructured.Unstructured{Object: obj}, opts)
	if err != nil {
		return nil, err
	}
	result := srlinuxv1.Srlinux{}
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), &result); err != nil {
		return nil, fmt.Errorf("failed to type assert return to Srlinux: %w", err)
	}
	return &result, nil
}

func (s *srlinuxClient) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return s.dInterface.Namespace(s.ns).Watch(ctx, opts)
}

func (s *srlinuxClient) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return s.dInterface.Namespace(s.ns).Delete(ctx, name, opts)
}

func (s *srlinuxClient) Update(ctx context.Context, obj *unstructured.Unstructured, opts metav1.UpdateOptions) (*srlinuxv1.Srlinux, error) {
	obj, err := s.dInterface.Namespace(s.ns).UpdateStatus(ctx, obj, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	result := srlinuxv1.Srlinux{}
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &result); err != nil {
		return nil, fmt.Errorf("failed to type assert return to Srlinux: %w", err)
	}
	return &result, nil
}

func (s *srlinuxClient) Unstructured(ctx context.Context, name string, opts metav1.GetOptions,
	subresources ...string,
) (*unstructured.Unstructured, error) {
	return s.dInterface.Namespace(s.ns).Get(ctx, name, opts, subresources...)
}
