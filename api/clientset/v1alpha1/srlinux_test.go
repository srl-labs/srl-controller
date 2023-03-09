// Copyright 2022 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package v1alpha1

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/h-fam/errdiff"
	srlinuxv1 "github.com/srl-labs/srl-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ktest "k8s.io/client-go/testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	restfake "k8s.io/client-go/rest/fake"
)

var (
	obj1 = &srlinuxv1.Srlinux{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Srlinux",
			APIVersion: "kne.srlinux.dev/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "obj1",
			Namespace:  "test",
			Generation: 1,
		},
		Status: srlinuxv1.SrlinuxStatus{},
		Spec: srlinuxv1.SrlinuxSpec{
			Config:        &srlinuxv1.NodeConfig{},
			NumInterfaces: 2,
			Model:         "fake",
			Version:       "1",
		},
	}

	obj2 = &srlinuxv1.Srlinux{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Srlinux",
			APIVersion: "kne.srlinux.dev/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "obj2",
			Namespace:  "test",
			Generation: 1,
		},
		Status: srlinuxv1.SrlinuxStatus{},
		Spec: srlinuxv1.SrlinuxSpec{
			Config:        &srlinuxv1.NodeConfig{},
			NumInterfaces: 2,
			Model:         "fake1",
			Version:       "2",
		},
	}

	// ignoreTypeMetaOpt is cmpopt option that is used to discard TypeMeta field
	// when comparing Srlinux structs. This is required since fakeRest server will never populate
	// those fields, and those fields may be present in the test's want object.
	ignoreTypeMetaOpt = cmpopts.IgnoreFields(srlinuxv1.Srlinux{}, "TypeMeta")
)

// setUp creates a Srlinux clientset and patches its rest and dynamic clients.
func setUp(t *testing.T) (*Clientset, *restfake.RESTClient) {
	t.Helper()

	gv := GV()

	fakeClient := &restfake.RESTClient{
		NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
		GroupVersion:         *gv,
		VersionedAPIPath:     GVR().Version,
		Err:                  nil,
		Req:                  &http.Request{},
		Client:               &http.Client{},
		Resp:                 &http.Response{},
	}

	cs, err := NewForConfig(&rest.Config{})
	if err != nil {
		t.Fatalf("NewForConfig() failed: %v", err)
	}

	// objects will be added to the object tracker when the fake dynamic interface is created,
	// this allows to unit test methods that require processing of unstructured data.
	objs := []runtime.Object{obj1, obj2}
	cs.restClient = fakeClient

	f := dynamicfake.NewSimpleDynamicClient(scheme.Scheme, objs...)
	f.PrependReactor("get", "*", func(action ktest.Action) (bool, runtime.Object, error) {
		gAction := action.(ktest.GetAction)
		switch gAction.GetName() {
		case "obj1":
			return true, obj1, nil
		case "obj2":
			return true, obj2, nil
		}

		return false, nil, nil
	})

	f.PrependReactor("update", "*", func(action ktest.Action) (bool, runtime.Object, error) {
		uAction, ok := action.(ktest.UpdateAction)
		if !ok {
			return false, nil, nil
		}

		uObj := uAction.GetObject().(*unstructured.Unstructured)
		sObj := &srlinuxv1.Srlinux{}

		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(uObj.Object, sObj); err != nil {
			return true, nil, fmt.Errorf("failed to convert object: %v", err)
		}

		if sObj.ObjectMeta.Name == "doesnotexist" {
			return true, nil, fmt.Errorf("doesnotexist")
		}

		return true, uAction.GetObject(), nil
	})

	cs.dInterface = f.Resource(GVR())

	return cs, fakeClient
}

func TestCreate(t *testing.T) {
	cs, fakeClient := setUp(t)

	tests := []struct {
		desc    string
		resp    *http.Response
		want    *srlinuxv1.Srlinux
		wantErr string
	}{{
		desc:    "Error",
		wantErr: "TEST ERROR",
	}, {
		desc: "Valid Node",
		resp: &http.Response{
			StatusCode: http.StatusOK,
		},
		want: obj1,
	}}

	for _, tt := range tests {
		fakeClient.Err = nil // set Err to nil on each case start
		// if we expect an error to be returned from the rest server
		// we set the fake rest client error to it
		// thus it will be returned for every rest call to that server.
		if tt.wantErr != "" {
			fakeClient.Err = fmt.Errorf(tt.wantErr)
		}

		fakeClient.Resp = tt.resp

		// rest unit tests keep no state on the server side
		// instead, whatever we want to be returned we populate as a resp.Body
		if tt.want != nil {
			b, _ := json.Marshal(tt.want)
			tt.resp.Body = io.NopCloser(bytes.NewReader(b))
		}

		t.Run(tt.desc, func(t *testing.T) {
			tc := cs.Srlinux("foo")
			got, err := tc.Create(context.Background(), tt.want)

			if s := errdiff.Substring(err, tt.wantErr); s != "" {
				t.Fatalf("unexpected error: %s", s)
			}

			if tt.wantErr != "" {
				return
			}

			if s := cmp.Diff(got, tt.want, ignoreTypeMetaOpt); s != "" {
				t.Fatalf("Create failed.\nGot: %+v\nWant: %+v\nDiff\n%s", got, tt.want, s)
			}
		})
	}
}

func TestList(t *testing.T) {
	cs, fakeClient := setUp(t)
	tests := []struct {
		desc    string
		resp    *http.Response
		want    *srlinuxv1.SrlinuxList
		wantErr string
	}{{
		desc:    "Error",
		wantErr: "TEST ERROR",
	}, {
		desc: "Valid Node",
		resp: &http.Response{
			StatusCode: http.StatusOK,
		},
		want: &srlinuxv1.SrlinuxList{
			Items: []srlinuxv1.Srlinux{*obj1, *obj2},
		},
	}}

	for _, tt := range tests {
		fakeClient.Err = nil

		if tt.wantErr != "" {
			fakeClient.Err = fmt.Errorf(tt.wantErr)
		}

		fakeClient.Resp = tt.resp

		if tt.want != nil {
			b, _ := json.Marshal(tt.want)
			tt.resp.Body = io.NopCloser(bytes.NewReader(b))
		}

		t.Run(tt.desc, func(t *testing.T) {
			tc := cs.Srlinux("foo")

			got, err := tc.List(context.Background(), metav1.ListOptions{})
			if s := errdiff.Substring(err, tt.wantErr); s != "" {
				t.Fatalf("unexpected error: %s", s)
			}

			if tt.wantErr != "" {
				return
			}

			if s := cmp.Diff(got, tt.want, ignoreTypeMetaOpt); s != "" {
				t.Fatalf("List failed.\nGot: %+v\nWant: %+v\nDiff\n%s", got, tt.want, s)
			}
		})
	}
}

func TestGet(t *testing.T) {
	cs, fakeClient := setUp(t)
	tests := []struct {
		desc    string
		resp    *http.Response
		want    *srlinuxv1.Srlinux
		wantErr string
	}{{
		desc:    "Error",
		wantErr: "TEST ERROR",
	}, {
		desc: "Valid Node",
		resp: &http.Response{
			StatusCode: http.StatusOK,
		},
		want: obj1,
	}}

	for _, tt := range tests {
		fakeClient.Err = nil

		if tt.wantErr != "" {
			fakeClient.Err = fmt.Errorf(tt.wantErr)
		}

		fakeClient.Resp = tt.resp

		if tt.want != nil {
			b, _ := json.Marshal(tt.want)
			tt.resp.Body = io.NopCloser(bytes.NewReader(b))
		}

		t.Run(tt.desc, func(t *testing.T) {
			tc := cs.Srlinux("foo")

			got, err := tc.Get(context.Background(), "test", metav1.GetOptions{})
			if s := errdiff.Substring(err, tt.wantErr); s != "" {
				t.Fatalf("unexpected error: %s", s)
			}

			if tt.wantErr != "" {
				return
			}

			if s := cmp.Diff(got, tt.want, ignoreTypeMetaOpt); s != "" {
				t.Fatalf("Get failed.\nGot: %+v\nWant: %+v\nDiff\n%s", got, tt.want, s)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	cs, fakeClient := setUp(t)
	tests := []struct {
		desc    string
		resp    *http.Response
		wantErr string
	}{{
		desc:    "Error",
		wantErr: "TEST ERROR",
	}, {
		desc: "Valid Node",
		resp: &http.Response{
			StatusCode: http.StatusOK,
		},
	}}

	for _, tt := range tests {
		fakeClient.Err = nil

		if tt.wantErr != "" {
			fakeClient.Err = fmt.Errorf(tt.wantErr)
		}

		fakeClient.Resp = tt.resp

		t.Run(tt.desc, func(t *testing.T) {
			tc := cs.Srlinux("foo")

			err := tc.Delete(context.Background(), "obj1", metav1.DeleteOptions{})
			if s := errdiff.Substring(err, tt.wantErr); s != "" {
				t.Fatalf("unexpected error: %s", s)
			}

			if tt.wantErr != "" {
				return
			}
		})
	}
}

func TestWatch(t *testing.T) {
	cs, fakeClient := setUp(t)
	tests := []struct {
		desc    string
		resp    *http.Response
		want    *watch.Event
		wantErr string
	}{{
		desc:    "Error",
		wantErr: "TEST ERROR",
	}}

	for _, tt := range tests {
		fakeClient.Err = nil

		if tt.wantErr != "" {
			fakeClient.Err = fmt.Errorf(tt.wantErr)
		}

		fakeClient.Resp = tt.resp

		if tt.want != nil {
			b, _ := json.Marshal(tt.want)
			tt.resp.Body = io.NopCloser(bytes.NewReader(b))
		}

		t.Run(tt.desc, func(t *testing.T) {
			tc := cs.Srlinux("foo")
			w, err := tc.Watch(context.Background(), metav1.ListOptions{})
			if s := errdiff.Substring(err, tt.wantErr); s != "" {
				t.Fatalf("unexpected error: %s", s)
			}

			if tt.wantErr != "" {
				return
			}

			got := <-w.ResultChan()
			if s := cmp.Diff(got, tt.want); s != "" {
				t.Fatalf("Watch failed.\nGot: %+v\nWant: %+v\nDiff\n%s", got, tt.want, s)
			}
		})
	}
}

func TestUnstructured(t *testing.T) {
	cs, _ := setUp(t)
	tests := []struct {
		desc    string
		in      string
		want    *srlinuxv1.Srlinux
		wantErr string
	}{{
		desc:    "Error",
		in:      "missingObj",
		wantErr: `"missingObj" not found`,
	}, {
		desc: "Valid Node 1",
		in:   obj1.GetObjectMeta().GetName(),
		want: obj1,
	}, {
		desc: "Valid Node 2",
		in:   obj2.GetObjectMeta().GetName(),
		want: obj2,
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			tc := cs.Srlinux("test")
			got, err := tc.Unstructured(context.Background(), tt.in, metav1.GetOptions{})
			if s := errdiff.Substring(err, tt.wantErr); s != "" {
				t.Fatalf("unexpected error: %s", s)
			}

			if tt.wantErr != "" {
				return
			}

			uObj1 := &srlinuxv1.Srlinux{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(got.Object, uObj1); err != nil {
				t.Fatalf("failed to turn response into a topology: %v", err)
			}

			if s := cmp.Diff(uObj1, tt.want); s != "" {
				t.Fatalf("Unstructured (%q) failed.\nGot: %+v\nWant: %+v\nDiff\n%s", tt.in, uObj1, tt.want, s)
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	cs, _ := setUp(t)
	tests := []struct {
		desc    string
		want    *srlinuxv1.Srlinux
		wantErr string
	}{{
		desc: "Error",
		want: &srlinuxv1.Srlinux{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Srlinux",
				APIVersion: "kne.srlinux.dev/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       "doesnotexist",
				Namespace:  "test",
				Generation: 1,
			},
		},
		wantErr: "doesnotexist",
	}, {
		desc: "Valid Node",
		want: obj1,
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			sc := cs.Srlinux("test")
			updateObj := tt.want.DeepCopy()
			updateObj.Spec.Version = "updated version"

			update, err := runtime.DefaultUnstructuredConverter.ToUnstructured(updateObj)
			if err != nil {
				t.Fatalf("failed to generate update: %v", err)
			}

			got, err := sc.Update(context.Background(), &unstructured.Unstructured{
				Object: update,
			}, metav1.UpdateOptions{})

			if s := errdiff.Substring(err, tt.wantErr); s != "" {
				t.Fatalf("unexpected error: %s", s)
			}

			if tt.wantErr != "" {
				return
			}

			if s := cmp.Diff(got, updateObj); s != "" {
				t.Fatalf("Update failed.\nGot: %+v\nWant: %+v\nDiff\n%s", got, updateObj, s)
			}
		})
	}
}
