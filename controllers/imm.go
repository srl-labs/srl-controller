// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package controllers

import srlinuxv1 "github.com/srl-labs/srl-controller/api/v1"

// imm holds the topology type IDs of an SR Linux line card (IMM).
type imm struct {
	cardType    int
	mdaType     int
	cpmCardType int
}

// immCatalog maps SR Linux line card (IMM) names to their topology type IDs.
//
// The IDs are taken from the slot_configuration and chassis_configuration of the
// containerlab SR Linux topology templates (github.com/srl-labs/containerlab,
// nodes/srl/topology/*.yml). Keys must match the enum on srlinuxv1.IMMType.
var immCatalog = map[srlinuxv1.IMMType]imm{
	"imm2-36-400g-sync-qsfpdd": {cardType: 43, mdaType: 199, cpmCardType: 41},
	"imm3-18-800g-qsfpdd":      {cardType: 33, mdaType: 13, cpmCardType: 184},
	"imm3-36-800g-osfp":        {cardType: 53, mdaType: 14, cpmCardType: 184},
	"imm3-36-800g-qsfpdd":      {cardType: 52, mdaType: 11, cpmCardType: 184},
	"imm3-36-800g-sync-qsfpdd": {cardType: 46, mdaType: 11, cpmCardType: 25},
	"imm36-400g-qsfpdd":        {cardType: 182, mdaType: 199, cpmCardType: 184},
	"imm60-100g-qsfp28":        {cardType: 183, mdaType: 219, cpmCardType: 184},
}
