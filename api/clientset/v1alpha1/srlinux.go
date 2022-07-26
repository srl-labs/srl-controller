// Clientset for SRLinux custom resource.
package v1alpha1

// note to my future self: see https://www.martin-helmich.de/en/blog/kubernetes-crd-client.html for details

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	typesv1alpha1 "github.com/srl-labs/srl-controller/api/types/v1alpha1"
)

// SrlinuxInterface provides access to the Srlinux CRD.
type SrlinuxInterface interface {
	List(ctx context.Context, opts metav1.ListOptions) (*typesv1alpha1.SrlinuxList, error)
	Get(ctx context.Context, name string, options metav1.GetOptions) (*typesv1alpha1.Srlinux, error)
	Create(ctx context.Context, srlinux *typesv1alpha1.Srlinux) (*typesv1alpha1.Srlinux, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	Unstructured(ctx context.Context, name string, opts metav1.GetOptions, subresources ...string) (*unstructured.Unstructured, error)
	Update(ctx context.Context, obj *unstructured.Unstructured, opts metav1.UpdateOptions) (*typesv1alpha1.Srlinux, error)
}

// Interface is the clientset interface for srlinux.
type Interface interface {
	Srlinux(namespace string) SrlinuxInterface
}

// Clientset is a client for the srlinux crds.
type Clientset struct {
	dInterface dynamic.NamespaceableResourceInterface
	restClient rest.Interface
}

var gvr = schema.GroupVersionResource{
	Group:    typesv1alpha1.GroupVersion.Group,
	Version:  typesv1alpha1.GroupVersion.Version,
	Resource: "srlinuxes",
}

// NewForConfig returns a new Clientset based on c.
func NewForConfig(c *rest.Config) (*Clientset, error) {
	config := *c
	config.ContentConfig.GroupVersion = &schema.GroupVersion{Group: gvr.Group, Version: gvr.Version}
	config.APIPath = "/apis"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	config.UserAgent = rest.DefaultKubernetesUserAgent()
	dClient, err := dynamic.NewForConfig(c)
	if err != nil {
		return nil, err
	}
	dInterface := dClient.Resource(gvr)
	rClient, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &Clientset{
		dInterface: dInterface,
		restClient: rClient,
	}, nil
}

func (c *Clientset) Srlinux(namespace string) SrlinuxInterface {
	return &srlinuxClient{
		dInterface: c.dInterface,
		restClient: c.restClient,
		ns:         namespace,
	}
}

type srlinuxClient struct {
	dInterface dynamic.NamespaceableResourceInterface
	restClient rest.Interface
	ns         string
}

func (s *srlinuxClient) List(ctx context.Context, opts metav1.ListOptions) (*typesv1alpha1.SrlinuxList, error) {
	result := typesv1alpha1.SrlinuxList{}
	err := s.restClient.
		Get().
		Namespace(s.ns).
		Resource(gvr.Resource).
		VersionedParams(&opts, scheme.ParameterCodec).
		Do(ctx).
		Into(&result)

	return &result, err
}

func (s *srlinuxClient) Get(ctx context.Context, name string, opts metav1.GetOptions) (*typesv1alpha1.Srlinux, error) {
	result := typesv1alpha1.Srlinux{}
	err := s.restClient.
		Get().
		Namespace(s.ns).
		Resource(gvr.Resource).
		Name(name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Do(ctx).
		Into(&result)

	return &result, err
}

func (s *srlinuxClient) Create(ctx context.Context, srlinux *typesv1alpha1.Srlinux) (*typesv1alpha1.Srlinux, error) {
	result := typesv1alpha1.Srlinux{}
	err := s.restClient.
		Post().
		Namespace(s.ns).
		Resource(gvr.Resource).
		Body(srlinux).
		Do(ctx).
		Into(&result)

	return &result, err
}

func (s *srlinuxClient) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return s.restClient.
		Get().
		Namespace(s.ns).
		Resource(gvr.Resource).
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch(ctx)
}

func (t *srlinuxClient) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return t.restClient.
		Delete().
		Namespace(t.ns).
		Resource(gvr.Resource).
		VersionedParams(&opts, scheme.ParameterCodec).
		Name(name).
		Do(ctx).
		Error()
}

func (s *srlinuxClient) Update(ctx context.Context, obj *unstructured.Unstructured, opts metav1.UpdateOptions) (*typesv1alpha1.Srlinux, error) {
	result := typesv1alpha1.Srlinux{}
	obj, err := s.dInterface.Namespace(s.ns).UpdateStatus(ctx, obj, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to type assert return to srlinux")
	}
	return &result, nil
}

func (s *srlinuxClient) Unstructured(ctx context.Context, name string, opts metav1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return s.dInterface.Namespace(s.ns).Get(ctx, name, opts, subresources...)
}

func init() {
	_ = typesv1alpha1.AddToScheme(scheme.Scheme)
}
