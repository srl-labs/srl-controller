package v1alpha1

import (
	"errors"
	"regexp"
	"strings"
)

// ErrVersionParse is an error which is raised when srlinux version is failed to parse.
var ErrVersionParse = errors.New("version parsing failed")

// SrlVersion represents an sr linux version as a set of fields.
type SrlVersion struct {
	Major  string `json:"major,omitempty"`
	Minor  string `json:"minor,omitempty"`
	Patch  string `json:"patch,omitempty"`
	Build  string `json:"build,omitempty"`
	Commit string `json:"commit,omitempty"`
}

func parseVersionString(s string) (*SrlVersion, error) {
	// for latest or missing tag we consider the version to be an engineering build
	// with major = 0
	if strings.ToLower(s) == "latest" || s == "" {
		return &SrlVersion{"0", "", "", "", ""}, nil
	}

	// https://regex101.com/r/eWS6Ms/1
	re := regexp.MustCompile(
		`^v?(?P<major>\d{1,3})\.(?P<minor>\d{1,2})\.?(?P<patch>\d{1,2})?-?(?P<build>\d{1,10})?-?(?P<commit>\S+)?`,
	)

	v := re.FindStringSubmatch(s)
	if v == nil {
		return nil, ErrVersionParse
	}

	return &SrlVersion{v[1], v[2], v[3], v[4], v[5]}, nil
}
