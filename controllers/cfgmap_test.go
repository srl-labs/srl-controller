// Copyright 2022 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package controllers

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	srlinuxv1 "github.com/srl-labs/srl-controller/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCreateConfigMaps(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	ns := "test-reboot-ns"

	fakeClient := fake.NewClientBuilder().WithScheme(clientgoscheme.Scheme).Build()
	reconciler := &SrlinuxReconciler{
		Client: fakeClient,
		Scheme: clientgoscheme.Scheme,
	}

	srl := &srlinuxv1.Srlinux{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-srl",
			Namespace: ns,
		},
		Spec: srlinuxv1.SrlinuxSpec{
			Config: &srlinuxv1.NodeConfig{
				Image: "ghcr.io/nokia/srlinux:latest",
			},
		},
	}

	// 1. First invocation creates all required ConfigMaps
	err := createConfigMaps(ctx, reconciler, srl, logr.Discard())
	g.Expect(err).NotTo(HaveOccurred())

	// 2. Verify topomac script ConfigMap
	topomacCM := &corev1.ConfigMap{}
	err = fakeClient.Get(ctx, types.NamespacedName{Name: topomacCfgMapName, Namespace: ns}, topomacCM)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(topomacCM.Data).To(HaveKey("topomac.sh"))
	topomacScript := topomacCM.Data["topomac.sh"]
	g.Expect(topomacScript).To(ContainSubstring("/etc/opt/srlinux/topology.yml"))
	g.Expect(topomacScript).To(ContainSubstring("cp -f"))
	g.Expect(topomacScript).To(ContainSubstring("__RANDMAC__"))

	// 3. Verify kne-entrypoint ConfigMap
	entrypointCM := &corev1.ConfigMap{}
	err = fakeClient.Get(ctx, types.NamespacedName{Name: entrypointCfgMapName, Namespace: ns}, entrypointCM)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(entrypointCM.Data).To(HaveKey("kne-entrypoint.sh"))
	entrypointScript := entrypointCM.Data["kne-entrypoint.sh"]
	g.Expect(entrypointScript).To(ContainSubstring("saved-ip-"))
	g.Expect(entrypointScript).To(ContainSubstring("saved-route-"))
	g.Expect(entrypointScript).To(ContainSubstring("dev mgmt0"))
	g.Expect(entrypointScript).To(ContainSubstring("config.json"))

	// 4. Verify variants ConfigMap
	variantsCM := &corev1.ConfigMap{}
	err = fakeClient.Get(ctx, types.NamespacedName{Name: variantsCfgMapName, Namespace: ns}, variantsCM)
	g.Expect(err).NotTo(HaveOccurred())

	// 5. Test idempotency: subsequent calls succeed without error
	err = createConfigMaps(ctx, reconciler, srl, logr.Discard())
	g.Expect(err).NotTo(HaveOccurred())
}

func TestVariantsManifests_DecodeCleanly(t *testing.T) {
	manifests := []string{
		"manifests/variants/topomac.yml",
		"manifests/variants/kne-entrypoint.yml",
		"manifests/variants/srl_variants.yml",
	}

	decoder := serializer.NewCodecFactory(clientgoscheme.Scheme).UniversalDecoder()

	for _, m := range manifests {
		t.Run(m, func(t *testing.T) {
			g := NewWithT(t)

			data, err := variantsFS.ReadFile(m)
			g.Expect(err).NotTo(HaveOccurred(), "failed to read embedded manifest %s", m)

			cfgMap := &corev1.ConfigMap{}
			err = runtime.DecodeInto(decoder, data, cfgMap)
			g.Expect(err).NotTo(HaveOccurred(), "failed to decode embedded manifest %s into ConfigMap", m)
			g.Expect(cfgMap.Data).NotTo(BeEmpty(), "decoded ConfigMap data must not be empty")
		})
	}
}
