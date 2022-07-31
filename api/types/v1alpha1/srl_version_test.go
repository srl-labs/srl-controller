package v1alpha1

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseVersionString(t *testing.T) {
	tests := []struct {
		desc string
		got  string
		want *SrlVersion
		err  error
	}{
		{
			desc: "maj, minor, patch",
			got:  "21.6.4",
			want: &SrlVersion{"21", "6", "4", "", ""},
		},
		{
			desc: "maj, minor",
			got:  "21.6",
			want: &SrlVersion{"21", "6", "", "", ""},
		},
		{
			desc: "maj, minor and extra string",
			got:  "21.6-test",
			want: &SrlVersion{"21", "6", "", "", "test"},
		},
		{
			desc: "maj, minor, patch and extra string",
			got:  "21.6.11-test",
			want: &SrlVersion{"21", "6", "11", "", "test"},
		},
		{
			desc: "maj, minor, patch, build and extra string",
			got:  "21.6.11-235-test",
			want: &SrlVersion{"21", "6", "11", "235", "test"},
		},
		{
			desc: "maj, minor, patch and build",
			got:  "21.6.11-235",
			want: &SrlVersion{"21", "6", "11", "235", ""},
		},
		{
			desc: "0.0",
			got:  "0.0",
			want: &SrlVersion{"0", "0", "", "", ""},
		},
		{
			desc: "0.0.0",
			got:  "0.0.0",
			want: &SrlVersion{"0", "0", "0", "", ""},
		},
		{
			desc: "0.0.0-34652",
			got:  "0.0.0-34652",
			want: &SrlVersion{"0", "0", "0", "34652", ""},
		},
		{
			desc: "latest",
			got:  "latest",
			want: &SrlVersion{"0", "", "", "", ""},
		},
		{
			desc: "empty",
			got:  "",
			want: &SrlVersion{"0", "", "", "", ""},
		},
		{
			desc: "invalid1",
			got:  "abcd",
			want: nil,
			err:  ErrVersionParse,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ver, err := parseVersionString(tt.got)
			if !errors.Is(err, tt.err) {
				t.Fatalf("got error '%v' but expected '%v'", err, tt.err)
			}

			if !cmp.Equal(ver, tt.want) {
				t.Fatalf(
					"%s: actual and expected inputs do not match\nactual: %+v\nexpected:%+v",
					tt.desc,
					ver,
					tt.want,
				)
			}
		},
		)
	}
}
