// Copyright 2022 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package v1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseVersionString(t *testing.T) {
	tests := []struct {
		desc string
		got  string
		want *SrlVersion
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
			desc: "version_0.0.0-34652",
			got:  "version_0.0.0-34652",
			want: &SrlVersion{"0", "0", "0", "34652", ""},
		},
		{
			desc: "latest",
			got:  "latest",
			want: &SrlVersion{"0", "", "", "", ""},
		},
		{
			desc: "ga",
			got:  "ga",
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
			want: &SrlVersion{"0", "", "", "", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ver := parseVersionString(tt.got)

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
