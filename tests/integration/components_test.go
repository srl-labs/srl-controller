package integration_controllers_test

import (
	"fmt"
	"strings"
	"testing"

	srlinuxv1 "github.com/srl-labs/srl-controller/api/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestComponentsValidation checks the spec.components rules enforced by the CRD schema.
func TestComponentsValidation(t *testing.T) {
	const ns = "components-validation"

	if err := k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		desc       string
		model      string
		components []srlinuxv1.Component
		wantErr    string // empty if valid
	}{{
		desc:       "typed and untyped line cards on ixr10e",
		model:      "ixr10e",
		components: []srlinuxv1.Component{{Slot: 1}, {Slot: 2, Type: "imm60-100g-qsfp28"}},
	}, {
		desc:       "untyped line cards on ixr10",
		model:      "ixr10",
		components: []srlinuxv1.Component{{Slot: 1}, {Slot: 2}},
	}, {
		desc:  "no components on fixed model",
		model: "ixrd2l",
	}, {
		desc:       "duplicate slot",
		model:      "ixr10e",
		components: []srlinuxv1.Component{{Slot: 2}, {Slot: 2}},
		wantErr:    "Duplicate value",
	}, {
		desc:       "slot 0",
		model:      "ixr10e",
		components: []srlinuxv1.Component{{Slot: 0}},
		wantErr:    "greater than or equal to 1",
	}, {
		desc:       "unknown line card type",
		model:      "ixr10e",
		components: []srlinuxv1.Component{{Slot: 1, Type: "imm-bogus"}},
		wantErr:    "Unsupported value",
	}, {
		desc:       "components on fixed model",
		model:      "ixrd3",
		components: []srlinuxv1.Component{{Slot: 1}},
		wantErr:    "only supported on modular models",
	}, {
		desc:       "components without model",
		components: []srlinuxv1.Component{{Slot: 1}},
		wantErr:    "only supported on modular models",
	}}

	for i, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			srl := &srlinuxv1.Srlinux{
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("srl-%d", i), Namespace: ns},
				Spec: srlinuxv1.SrlinuxSpec{
					Config:     &srlinuxv1.NodeConfig{Image: testImageName},
					Model:      tt.model,
					Components: tt.components,
				},
			}

			err := k8sClient.Create(ctx, srl)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Create() unexpected error: %v", err)
				}

				return
			}

			if !apierrors.IsInvalid(err) || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Create() error = %v, want Invalid containing %q", err, tt.wantErr)
			}
		})
	}
}
