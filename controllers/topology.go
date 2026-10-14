// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package controllers

import (
	"bytes"
	"context"
	"fmt"
	"strconv"

	"github.com/go-logr/logr"
	srlinuxv1 "github.com/srl-labs/srl-controller/api/v1"
	"go.yaml.in/yaml/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	topologyCfgMapSuffix = "-topology"

	chassisConfigKey = "chassis_configuration"
	slotConfigKey    = "slot_configuration"
	cpmCardTypeKey   = "cpm_card_type"
	cardTypeKey      = "card_type"
	mdaTypeKey       = "mda_type"

	yamlIndent = 2
)

// topologyCfgMapName returns the name of the per-node topology config map.
func topologyCfgMapName(s *srlinuxv1.Srlinux) string {
	return s.Name + topologyCfgMapSuffix
}

// createTopologyCfgMap creates or updates the per-node topology config map holding the model's
// topology template from variants with its slot configuration replaced by the node's components.
// It is a no-op when the node has no components.
func createTopologyCfgMap(
	ctx context.Context,
	r *SrlinuxReconciler,
	s *srlinuxv1.Srlinux,
	variants *corev1.ConfigMap,
	log logr.Logger,
) error {
	if len(s.Spec.Components) == 0 {
		return nil
	}

	model := s.Spec.GetModel()

	base, ok := variants.Data[model]
	if !ok {
		return fmt.Errorf("model %q not found in config map %s", model, variantsCfgMapName)
	}

	topo, err := renderTopology(base, s.Spec.Components)
	if err != nil {
		return fmt.Errorf("rendering topology for model %q: %w", model, err)
	}

	cfgMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      topologyCfgMapName(s),
			Namespace: s.Namespace,
		},
	}

	res, err := controllerutil.CreateOrUpdate(ctx, r.Client, cfgMap, func() error {
		cfgMap.Data = map[string]string{variantsTemplateTempName: topo}

		return ctrl.SetControllerReference(s, cfgMap, r.Scheme)
	})
	if err != nil {
		return err
	}

	log.Info("topology configmap reconciled", "name", cfgMap.Name, "operation", res)

	return nil
}

// renderTopology returns the topology template base with its slot configuration replaced by
// components. A component without a type uses the line card of the template's lowest slot.
// The chassis CPM card type is taken from the first component with a type, if any.
// Slot numbers are validated by the CRD.
func renderTopology(base string, components []srlinuxv1.Component) (string, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(base), &doc); err != nil {
		return "", err
	}

	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 {
		return "", fmt.Errorf("topology template is not a YAML document")
	}

	root := doc.Content[0]
	slots := mappingValue(root, slotConfigKey)
	cpm := mappingValue(mappingValue(root, chassisConfigKey), cpmCardTypeKey)

	if slots == nil || len(slots.Content) == 0 || cpm == nil {
		return "", fmt.Errorf("topology template needs %s and %s.%s", slotConfigKey, chassisConfigKey, cpmCardTypeKey)
	}

	defaultCard, err := lowestSlot(slots)
	if err != nil {
		return "", err
	}

	chassisCPM := ""
	newSlots := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}

	for _, c := range components {
		card := defaultCard

		if c.Type != "" {
			m, ok := immCatalog[c.Type]
			if !ok {
				return "", fmt.Errorf("unknown line card type %q", c.Type)
			}

			card = slotCard{cardType: intNode(m.cardType), mdaType: intNode(m.mdaType)}

			if chassisCPM == "" {
				chassisCPM = strconv.Itoa(m.cpmCardType)
			}
		}

		newSlots.Content = append(newSlots.Content,
			intNode(int(c.Slot)),
			&yaml.Node{
				Kind: yaml.MappingNode,
				Tag:  "!!map",
				Content: []*yaml.Node{
					keyNode(cardTypeKey), card.cardType,
					keyNode(mdaTypeKey), card.mdaType,
				},
			},
		)
	}

	*slots = *newSlots

	if chassisCPM != "" {
		cpm.Value = chassisCPM
	}

	var buf bytes.Buffer

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(yamlIndent)

	if err := enc.Encode(&doc); err != nil {
		return "", err
	}

	if err := enc.Close(); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// slotCard holds the card and MDA type nodes of a slot.
type slotCard struct {
	cardType *yaml.Node
	mdaType  *yaml.Node
}

// lowestSlot returns the card and MDA type of the lowest numbered slot in a slot_configuration mapping.
func lowestSlot(slots *yaml.Node) (slotCard, error) {
	var (
		card   slotCard
		lowest = -1
	)

	for i := 0; i+1 < len(slots.Content); i += 2 {
		n, err := strconv.Atoi(slots.Content[i].Value)
		if err != nil {
			return slotCard{}, fmt.Errorf("invalid slot %q: %w", slots.Content[i].Value, err)
		}

		if lowest != -1 && n >= lowest {
			continue
		}

		v := slots.Content[i+1]

		ct, mda := mappingValue(v, cardTypeKey), mappingValue(v, mdaTypeKey)
		if ct == nil || mda == nil {
			return slotCard{}, fmt.Errorf("slot %d has no %s or %s", n, cardTypeKey, mdaTypeKey)
		}

		lowest = n
		card = slotCard{cardType: ct, mdaType: mda}
	}

	return card, nil
}

// mappingValue returns the value node for key in mapping node m, or nil if absent.
func mappingValue(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}

	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}

	return nil
}

// keyNode returns a double-quoted string key node, matching the style of the variant templates.
func keyNode(s string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s, Style: yaml.DoubleQuotedStyle}
}

func intNode(i int) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(i)}
}
