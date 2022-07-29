package v1alpha1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
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
