// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package controllers

import (
	"context"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	srlinuxv1 "github.com/srl-labs/srl-controller/api/v1"
	"go.yaml.in/yaml/v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type testSlot struct {
	CardType int `yaml:"card_type"`
	MDAType  int `yaml:"mda_type"`
}

type testTopology struct {
	Chassis struct {
		ChassisType int    `yaml:"chassis_type"`
		BaseMAC     string `yaml:"base_mac"`
		CPMCardType int    `yaml:"cpm_card_type"`
	} `yaml:"chassis_configuration"`
	Slots map[int]testSlot `yaml:"slot_configuration"`
}

// variantTemplates returns the embedded topology templates keyed by model.
func variantTemplates(t *testing.T) map[string]string {
	t.Helper()

	data, err := variantsFS.ReadFile("manifests/variants/srl_variants.yml")
	if err != nil {
		t.Fatal(err)
	}

	cm := &corev1.ConfigMap{}
	if err := runtime.DecodeInto(serializer.NewCodecFactory(clientgoscheme.Scheme).UniversalDecoder(), data, cm); err != nil {
		t.Fatal(err)
	}

	return cm.Data
}

// variantTemplate returns the embedded topology template for model.
func variantTemplate(t *testing.T, model string) string {
	t.Helper()

	tmpl, ok := variantTemplates(t)[model]
	if !ok {
		t.Fatalf("model %q not in variants", model)
	}

	return tmpl
}

func TestRenderTopology(t *testing.T) {
	base := variantTemplate(t, "ixr10e")

	tests := []struct {
		desc        string
		components  []srlinuxv1.Component
		wantCPM     int
		wantSlots   map[int]testSlot
		wantErrText string
	}{{
		desc:       "derived slots copy the template line card",
		components: []srlinuxv1.Component{{Slot: 1}, {Slot: 2}},
		wantCPM:    184,
		wantSlots:  map[int]testSlot{1: {182, 199}, 2: {182, 199}},
	}, {
		desc:       "slot 1 unpopulated",
		components: []srlinuxv1.Component{{Slot: 2}},
		wantCPM:    184,
		wantSlots:  map[int]testSlot{2: {182, 199}},
	}, {
		desc: "mixed explicit types",
		components: []srlinuxv1.Component{
			{Slot: 1, Type: "imm36-400g-qsfpdd"},
			{Slot: 2, Type: "imm60-100g-qsfp28"},
		},
		wantCPM:   184,
		wantSlots: map[int]testSlot{1: {182, 199}, 2: {183, 219}},
	}, {
		desc: "CPM taken from first explicit type",
		components: []srlinuxv1.Component{
			{Slot: 1},
			{Slot: 3, Type: "imm2-36-400g-sync-qsfpdd"},
		},
		wantCPM:   41,
		wantSlots: map[int]testSlot{1: {182, 199}, 3: {43, 199}},
	}, {
		desc: "first explicit type wins when CPMs differ",
		components: []srlinuxv1.Component{
			{Slot: 2, Type: "imm3-36-800g-sync-qsfpdd"},
			{Slot: 1, Type: "imm2-36-400g-sync-qsfpdd"},
		},
		wantCPM:   25,
		wantSlots: map[int]testSlot{1: {43, 199}, 2: {46, 11}},
	}, {
		desc:        "unknown type",
		components:  []srlinuxv1.Component{{Slot: 1, Type: "imm-bogus"}},
		wantErrText: `unknown line card type "imm-bogus"`,
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got, err := renderTopology(base, tt.components)
			if tt.wantErrText != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErrText) {
					t.Fatalf("renderTopology() error = %v, want %q", err, tt.wantErrText)
				}

				return
			}

			if err != nil {
				t.Fatalf("renderTopology() unexpected error: %v", err)
			}

			var topo testTopology
			if err := yaml.Unmarshal([]byte(got), &topo); err != nil {
				t.Fatalf("rendered topology is not valid YAML: %v\n%s", err, got)
			}

			if topo.Chassis.ChassisType != 4 {
				t.Errorf("chassis_type = %d, want 4", topo.Chassis.ChassisType)
			}

			if topo.Chassis.BaseMAC != "02:__RANDMAC__:00:00:00" {
				t.Errorf("base_mac = %q, want template placeholder preserved", topo.Chassis.BaseMAC)
			}

			if topo.Chassis.CPMCardType != tt.wantCPM {
				t.Errorf("cpm_card_type = %d, want %d", topo.Chassis.CPMCardType, tt.wantCPM)
			}

			if diff := cmp.Diff(tt.wantSlots, topo.Slots); diff != "" {
				t.Errorf("slot_configuration mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestReconcileTopologyConfigMap(t *testing.T) {
	tests := []struct {
		desc       string
		components []srlinuxv1.Component
		wantCfgMap string
		wantKey    string
	}{{
		desc:       "no components uses shared variants",
		wantCfgMap: variantsCfgMapName,
		wantKey:    "ixr10e",
	}, {
		desc:       "components use per-node topology",
		components: []srlinuxv1.Component{{Slot: 1}, {Slot: 2}},
		wantCfgMap: defaultCRName + topologyCfgMapSuffix,
		wantKey:    variantsTemplateTempName,
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			srl := &srlinuxv1.Srlinux{
				ObjectMeta: ctrl.ObjectMeta{Name: defaultCRName, Namespace: defaultNamespace},
				Spec: srlinuxv1.SrlinuxSpec{
					Config:     &srlinuxv1.NodeConfig{Image: defaultSrlinuxImage},
					Model:      "ixr10e",
					Components: tt.components,
				},
			}

			c := fake.NewClientBuilder().
				WithRuntimeObjects(srl).
				WithStatusSubresource(&srlinuxv1.Srlinux{}).
				Build()
			r := SrlinuxReconciler{Scheme: clientgoscheme.Scheme, Client: c}

			if _, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName}); err != nil {
				t.Fatalf("Reconcile() error: %v", err)
			}

			pod := &corev1.Pod{}
			if err := c.Get(ctx, namespacedName, pod); err != nil {
				t.Fatalf("pod not created: %v", err)
			}

			var vol *corev1.Volume

			for i := range pod.Spec.Volumes {
				if pod.Spec.Volumes[i].Name == variantsVolName {
					vol = &pod.Spec.Volumes[i]
				}
			}

			if vol == nil || vol.ConfigMap == nil {
				t.Fatalf("pod has no %s config map volume", variantsVolName)
			}

			if got := vol.ConfigMap.Name; got != tt.wantCfgMap {
				t.Errorf("variants volume config map = %q, want %q", got, tt.wantCfgMap)
			}

			if got := vol.ConfigMap.Items[0].Key; got != tt.wantKey {
				t.Errorf("variants volume key = %q, want %q", got, tt.wantKey)
			}

			topoCM := &corev1.ConfigMap{}
			err := c.Get(ctx, types.NamespacedName{Name: defaultCRName + topologyCfgMapSuffix, Namespace: defaultNamespace}, topoCM)

			if len(tt.components) == 0 {
				if err == nil {
					t.Errorf("unexpected topology config map created")
				}

				return
			}

			if err != nil {
				t.Fatalf("topology config map not created: %v", err)
			}

			if _, ok := topoCM.Data[variantsTemplateTempName]; !ok {
				t.Errorf("topology config map missing key %q", variantsTemplateTempName)
			}

			if len(topoCM.OwnerReferences) != 1 || topoCM.OwnerReferences[0].Name != defaultCRName {
				t.Errorf("topology config map owner references = %v, want Srlinux %q", topoCM.OwnerReferences, defaultCRName)
			}
		})
	}
}

// TestCRDEnumMatchesCatalog guards against the CRD enum and the catalog drifting apart,
// e.g. regenerating the catalog without running `make manifests`.
// The remaining CRD validation rules are exercised against a real API server in tests/integration.
func TestCRDEnumMatchesCatalog(t *testing.T) {
	b, err := os.ReadFile("../config/crd/bases/kne.srlinux.dev_srlinuxes.yaml")
	if err != nil {
		t.Fatal(err)
	}

	var crd struct {
		Spec struct {
			Versions []struct {
				Schema struct {
					OpenAPIV3Schema struct {
						Properties struct {
							Spec struct {
								Properties struct {
									Components struct {
										Items struct {
											Properties struct {
												Type struct {
													Enum []string `yaml:"enum"`
												} `yaml:"type"`
											} `yaml:"properties"`
										} `yaml:"items"`
									} `yaml:"components"`
								} `yaml:"properties"`
							} `yaml:"spec"`
						} `yaml:"properties"`
					} `yaml:"openAPIV3Schema"`
				} `yaml:"schema"`
			} `yaml:"versions"`
		} `yaml:"spec"`
	}

	if err := yaml.Unmarshal(b, &crd); err != nil {
		t.Fatal(err)
	}

	if len(crd.Spec.Versions) != 1 {
		t.Fatalf("CRD has %d versions, want 1", len(crd.Spec.Versions))
	}

	got := crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties.Spec.Properties.Components.Items.Properties.Type.Enum
	sort.Strings(got)

	want := make([]string, 0, len(immCatalog))
	for k := range immCatalog {
		want = append(want, string(k))
	}

	sort.Strings(want)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("CRD components[].type enum does not match immCatalog (-want +got):\n%s", diff)
	}
}

func TestRenderTopologyCustomTemplates(t *testing.T) {
	// Slots listed out of order (3, 1, 2): slot 1 is the lowest and supplies the default line card.
	const multiSlot = `chassis_configuration:
  "chassis_type": 4
  "base_mac": 02:__RANDMAC__:00:00:00
  "cpm_card_type": 184
slot_configuration:
  3:
    "card_type": 183
    "mda_type": 219
  1:
    "card_type": 182
    "mda_type": 199
  2:
    "card_type": 52
    "mda_type": 11
`

	tests := []struct {
		desc       string
		base       string
		components []srlinuxv1.Component
		wantCPM    int
		wantSlots  map[int]testSlot
	}{{
		desc:       "default line card from lowest slot",
		base:       multiSlot,
		components: []srlinuxv1.Component{{Slot: 5}},
		wantCPM:    184,
		wantSlots:  map[int]testSlot{5: {182, 199}},
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got, err := renderTopology(tt.base, tt.components)
			if err != nil {
				t.Fatalf("renderTopology() unexpected error: %v", err)
			}

			var topo testTopology
			if err := yaml.Unmarshal([]byte(got), &topo); err != nil {
				t.Fatalf("rendered topology is not valid YAML: %v\n%s", err, got)
			}

			if topo.Chassis.CPMCardType != tt.wantCPM {
				t.Errorf("cpm_card_type = %d, want %d", topo.Chassis.CPMCardType, tt.wantCPM)
			}

			if diff := cmp.Diff(tt.wantSlots, topo.Slots); diff != "" {
				t.Errorf("slot_configuration mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRenderTopologyErrors(t *testing.T) {
	tests := []struct {
		desc        string
		base        string
		components  []srlinuxv1.Component
		wantErrText string
	}{{
		desc:        "invalid YAML",
		base:        "slot_configuration: [",
		wantErrText: "yaml",
	}, {
		desc:        "not a mapping",
		base:        "- a\n- b\n",
		wantErrText: "needs slot_configuration",
	}, {
		desc:        "empty document",
		base:        "",
		wantErrText: "not a YAML document",
	}, {
		desc:        "no slot_configuration",
		base:        "chassis_configuration:\n  \"cpm_card_type\": 184\n",
		wantErrText: "needs slot_configuration",
	}, {
		desc:        "empty slot_configuration",
		base:        "chassis_configuration:\n  \"cpm_card_type\": 184\nslot_configuration: {}\n",
		wantErrText: "needs slot_configuration",
	}, {
		desc:        "no cpm_card_type",
		base:        "chassis_configuration:\n  \"chassis_type\": 4\nslot_configuration:\n  1:\n    \"card_type\": 1\n    \"mda_type\": 2\n",
		wantErrText: "needs slot_configuration and chassis_configuration.cpm_card_type",
	}, {
		desc:        "non-numeric slot",
		base:        "chassis_configuration:\n  \"cpm_card_type\": 184\nslot_configuration:\n  x:\n    \"card_type\": 1\n    \"mda_type\": 2\n",
		wantErrText: `invalid slot "x"`,
	}, {
		desc:        "slot without mda_type",
		base:        "chassis_configuration:\n  \"cpm_card_type\": 184\nslot_configuration:\n  1:\n    \"card_type\": 1\n",
		wantErrText: "slot 1 has no card_type or mda_type",
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			components := tt.components
			if components == nil {
				components = []srlinuxv1.Component{{Slot: 1}}
			}

			_, err := renderTopology(tt.base, components)
			if err == nil || !strings.Contains(err.Error(), tt.wantErrText) {
				t.Fatalf("renderTopology() error = %v, want %q", err, tt.wantErrText)
			}
		})
	}
}

// TestRenderTopologyAllVariants renders every embedded model with two derived slots,
// guarding against variant template changes the renderer cannot handle.
func TestRenderTopologyAllVariants(t *testing.T) {
	for model, base := range variantTemplates(t) {
		t.Run(model, func(t *testing.T) {
			var orig testTopology
			if err := yaml.Unmarshal([]byte(base), &orig); err != nil {
				t.Fatalf("variant template is not valid YAML: %v", err)
			}

			got, err := renderTopology(base, []srlinuxv1.Component{{Slot: 1}, {Slot: 2}})
			if err != nil {
				t.Fatalf("renderTopology() unexpected error: %v", err)
			}

			var topo testTopology
			if err := yaml.Unmarshal([]byte(got), &topo); err != nil {
				t.Fatalf("rendered topology is not valid YAML: %v\n%s", err, got)
			}

			if diff := cmp.Diff(orig.Chassis, topo.Chassis); diff != "" {
				t.Errorf("chassis_configuration changed (-want +got):\n%s", diff)
			}

			want := map[int]testSlot{1: orig.Slots[1], 2: orig.Slots[1]}
			if diff := cmp.Diff(want, topo.Slots); diff != "" {
				t.Errorf("slot_configuration mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestReconcileTopologyConfigMapErrors(t *testing.T) {
	tests := []struct {
		desc        string
		model       string
		components  []srlinuxv1.Component
		wantErrText string
	}{{
		desc:        "model not in variants",
		model:       "bogus",
		components:  []srlinuxv1.Component{{Slot: 1}},
		wantErrText: `model "bogus" not found`,
	}, {
		// The CRD enum rejects unknown types on a real API server; the fake client does not.
		desc:        "unknown line card type",
		model:       "ixr10e",
		components:  []srlinuxv1.Component{{Slot: 1, Type: "imm-bogus"}},
		wantErrText: `unknown line card type "imm-bogus"`,
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			srl := &srlinuxv1.Srlinux{
				ObjectMeta: ctrl.ObjectMeta{Name: defaultCRName, Namespace: defaultNamespace},
				Spec: srlinuxv1.SrlinuxSpec{
					Config:     &srlinuxv1.NodeConfig{Image: defaultSrlinuxImage},
					Model:      tt.model,
					Components: tt.components,
				},
			}

			c := fake.NewClientBuilder().
				WithRuntimeObjects(srl).
				WithStatusSubresource(&srlinuxv1.Srlinux{}).
				Build()
			r := SrlinuxReconciler{Scheme: clientgoscheme.Scheme, Client: c}

			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName})
			if err == nil || !strings.Contains(err.Error(), tt.wantErrText) {
				t.Fatalf("Reconcile() error = %v, want %q", err, tt.wantErrText)
			}

			if err := c.Get(ctx, namespacedName, &corev1.Pod{}); err == nil {
				t.Errorf("pod created despite reconcile error")
			}
		})
	}
}

func TestReconcileTopologyConfigMapUpdatesStale(t *testing.T) {
	srl := &srlinuxv1.Srlinux{
		ObjectMeta: ctrl.ObjectMeta{Name: defaultCRName, Namespace: defaultNamespace},
		Spec: srlinuxv1.SrlinuxSpec{
			Config:     &srlinuxv1.NodeConfig{Image: defaultSrlinuxImage},
			Model:      "ixr10e",
			Components: []srlinuxv1.Component{{Slot: 2}},
		},
	}
	stale := &corev1.ConfigMap{
		ObjectMeta: ctrl.ObjectMeta{Name: defaultCRName + topologyCfgMapSuffix, Namespace: defaultNamespace},
		Data:       map[string]string{variantsTemplateTempName: "stale"},
	}

	c := fake.NewClientBuilder().
		WithRuntimeObjects(srl, stale).
		WithStatusSubresource(&srlinuxv1.Srlinux{}).
		Build()
	r := SrlinuxReconciler{Scheme: clientgoscheme.Scheme, Client: c}

	if _, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName}); err != nil {
		t.Fatalf("Reconcile() error: %v", err)
	}

	got := &corev1.ConfigMap{}
	if err := c.Get(ctx, types.NamespacedName{Name: stale.Name, Namespace: defaultNamespace}, got); err != nil {
		t.Fatal(err)
	}

	var topo testTopology
	if err := yaml.Unmarshal([]byte(got.Data[variantsTemplateTempName]), &topo); err != nil {
		t.Fatalf("topology config map data is not valid YAML: %v", err)
	}

	if diff := cmp.Diff(map[int]testSlot{2: {182, 199}}, topo.Slots); diff != "" {
		t.Errorf("slot_configuration mismatch (-want +got):\n%s", diff)
	}

	if len(got.OwnerReferences) != 1 || got.OwnerReferences[0].Name != defaultCRName {
		t.Errorf("owner references = %v, want Srlinux %q", got.OwnerReferences, defaultCRName)
	}
}

// TestReconcileTopologyConfigMapStaleCache checks that the topology config map is rendered from the
// variants config map the controller just created, even when reads do not yet return it, as with a
// stale informer cache.
func TestReconcileTopologyConfigMapStaleCache(t *testing.T) {
	srl := &srlinuxv1.Srlinux{
		ObjectMeta: ctrl.ObjectMeta{Name: defaultCRName, Namespace: defaultNamespace},
		Spec: srlinuxv1.SrlinuxSpec{
			Config:     &srlinuxv1.NodeConfig{Image: defaultSrlinuxImage},
			Model:      "ixr10e",
			Components: []srlinuxv1.Component{{Slot: 1}, {Slot: 2}},
		},
	}

	c := fake.NewClientBuilder().
		WithRuntimeObjects(srl).
		WithStatusSubresource(&srlinuxv1.Srlinux{}).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				if key.Name == variantsCfgMapName {
					return apierrors.NewNotFound(corev1.Resource("configmaps"), key.Name)
				}

				return c.Get(ctx, key, obj, opts...)
			},
		}).
		Build()
	r := SrlinuxReconciler{Scheme: clientgoscheme.Scheme, Client: c}

	if _, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName}); err != nil {
		t.Fatalf("Reconcile() error: %v", err)
	}

	got := &corev1.ConfigMap{}
	if err := c.Get(ctx, types.NamespacedName{Name: defaultCRName + topologyCfgMapSuffix, Namespace: defaultNamespace}, got); err != nil {
		t.Fatalf("topology config map not created: %v", err)
	}

	var topo testTopology
	if err := yaml.Unmarshal([]byte(got.Data[variantsTemplateTempName]), &topo); err != nil {
		t.Fatalf("topology config map data is not valid YAML: %v", err)
	}

	if diff := cmp.Diff(map[int]testSlot{1: {182, 199}, 2: {182, 199}}, topo.Slots); diff != "" {
		t.Errorf("slot_configuration mismatch (-want +got):\n%s", diff)
	}
}
