// Copyright 2022 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package v1

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
)

func TestGetImage(t *testing.T) {
	tests := []struct {
		desc string
		spec *SrlinuxSpec
		want string
	}{
		{
			desc: "no image, no version, default applies",
			spec: &SrlinuxSpec{},
			want: defaultSRLinuxImageName,
		},
		{
			desc: "image with valid tag",
			spec: &SrlinuxSpec{
				Config: &NodeConfig{
					Image: "ghcr.io/nokia/srlinux:22.6.1",
				},
			},
			want: "ghcr.io/nokia/srlinux:22.6.1",
		},
		{
			desc: "image undefined, version present",
			spec: &SrlinuxSpec{
				Version: "21.11.1",
			},
			want: defaultSRLinuxImageName + ":21.11.1",
		},
		{
			desc: "image without tag",
			spec: &SrlinuxSpec{
				Config: &NodeConfig{
					Image: "ghcr.io/nokia/srlinux",
				},
			},
			want: "ghcr.io/nokia/srlinux",
		},
		{
			desc: "image with latest tag",
			spec: &SrlinuxSpec{
				Config: &NodeConfig{
					Image: "ghcr.io/nokia/srlinux:latest",
				},
			},
			want: "ghcr.io/nokia/srlinux:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			img := tt.spec.GetImage()

			if !cmp.Equal(img, tt.want) {
				t.Fatalf(
					"%s: actual and expected inputs do not match\nactual: %+v\nexpected:%+v",
					tt.desc,
					img,
					tt.want,
				)
			}
		},
		)
	}
}

func TestGetImageVersion(t *testing.T) {
	tests := []struct {
		desc string
		spec *SrlinuxSpec
		want *SrlVersion
		err  error
	}{
		{
			desc: "valid version is present",
			spec: &SrlinuxSpec{
				Version: "21.11.1",
				Config:  &NodeConfig{Image: "ghcr.io/nokia/srlinux:somever"},
			},
			want: &SrlVersion{"21", "11", "1", "", ""},
		},
		{
			desc: "invalid version is present",
			spec: &SrlinuxSpec{
				Version: "abc",
				Config:  &NodeConfig{Image: "ghcr.io/nokia/srlinux:somever"},
			},
			err: ErrVersionParse,
		},
		{
			desc: "version is not present, valid image tag is given",
			spec: &SrlinuxSpec{
				Config: &NodeConfig{Image: "ghcr.io/nokia/srlinux:21.11.1"},
			},
			want: &SrlVersion{"21", "11", "1", "", ""},
		},
		{
			desc: "version is not present, invalid image tag is given",
			spec: &SrlinuxSpec{
				Config: &NodeConfig{Image: "ghcr.io/nokia/srlinux:somesrl"},
			},
			err: ErrVersionParse,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			v, err := tt.spec.GetImageVersion()

			if !errors.Is(err, tt.err) {
				t.Fatalf("got error '%v' but expected '%v'", err, tt.err)
			}

			if !cmp.Equal(v, tt.want) {
				t.Fatalf(
					"%s: actual and expected inputs do not match\nactual: %+v\nexpected:%+v",
					tt.desc,
					v,
					tt.want,
				)
			}
		},
		)
	}
}

func TestInitVersion(t *testing.T) {
	tests := []struct {
		desc    string
		version *SrlVersion
		secret  *corev1.Secret
		want    string
	}{
		{
			desc:    "secret key matches srl version",
			version: &SrlVersion{"22", "3", "", "", ""},
			secret: &corev1.Secret{
				Data: map[string][]byte{
					"22-3.key": nil,
					"all.key":  nil,
				},
			},
			want: "22-3.key",
		},
		{
			desc:    "wildcard secret key matches srl version",
			version: &SrlVersion{"22", "3", "", "", ""},
			secret: &corev1.Secret{
				Data: map[string][]byte{
					"22-6.key": nil,
					"all.key":  nil,
				},
			},
			want: "all.key",
		},
		{
			desc:    "secret does not exist",
			version: &SrlVersion{"22", "3", "", "", ""},
			secret:  nil,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			srl := &Srlinux{}
			srl.InitLicenseKey(context.TODO(), tt.secret, tt.version)

			if !cmp.Equal(srl.LicenseKey, tt.want) {
				t.Fatalf(
					"%s: actual and expected inputs do not match\nactual: %+v\nexpected:%+v",
					tt.desc,
					srl.LicenseKey,
					tt.want,
				)
			}
		},
		)
	}
}

func TestGetConstraints(t *testing.T) {
	tests := []struct {
		desc string
		spec *SrlinuxSpec
		want map[string]string
	}{
		{
			desc: "no constraints, default applies",
			spec: &SrlinuxSpec{},
			want: defaultConstraints,
		},
		{
			desc: "constraints are present",
			spec: &SrlinuxSpec{
				Constraints: map[string]string{"cpu": "2", "memory": "4Gi"},
			},

			want: map[string]string{"cpu": "2", "memory": "4Gi"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			img := tt.spec.GetConstraints()

			if !cmp.Equal(img, tt.want) {
				t.Fatalf(
					"%s: actual and expected inputs do not match\nactual: %+v\nexpected:%+v",
					tt.desc,
					img,
					tt.want,
				)
			}
		},
		)
	}
}

func TestGetModel(t *testing.T) {
	tests := []struct {
		desc string
		spec *SrlinuxSpec
		want string
	}{
		{
			desc: "no model specified, default applies",
			spec: &SrlinuxSpec{},
			want: defaultSrlinuxVariant,
		},
		{
			desc: "model is present",
			spec: &SrlinuxSpec{
				Model: "ixr-10e",
			},
			want: "ixr-10e",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			img := tt.spec.GetModel()

			if !cmp.Equal(img, tt.want) {
				t.Fatalf(
					"%s: actual and expected inputs do not match\nactual: %+v\nexpected:%+v",
					tt.desc,
					img,
					tt.want,
				)
			}
		},
		)
	}
}
