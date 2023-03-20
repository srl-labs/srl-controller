// Copyright 2023 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package e2e_controllers_test

import (
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	srlinuxv1 "github.com/srl-labs/srl-controller/api/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	SrlinuxNamespace      = "test"
	SrlinuxName           = "test-srlinux"
	testImageName         = "ghcr.io/nokia/srlinux:latest"
	defaultImageName      = "ghcr.io/nokia/srlinux:latest"
	srlinuxMaxStartupTime = 3 * time.Minute
)

var namespacedName = types.NamespacedName{Name: SrlinuxName, Namespace: SrlinuxNamespace}

func TestSrlinuxReconciler(t *testing.T) {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: SrlinuxNamespace,
		},
	}

	setup := func(t *testing.T, g *WithT) *corev1.Namespace {
		t.Helper()

		t.Log("Creating the namespace")
		g.Expect(k8sClient.Create(ctx, namespace)).Should(Succeed())

		return namespace
	}

	teardown := func(t *testing.T, _ *WithT) {
		t.Helper()

		t.Log("Deleting the namespace")

		_ = k8sClient.Delete(ctx, namespace)
	}

	t.Run("Should reconcile a Srlinux custom resource", func(t *testing.T) {
		g := NewWithT(t)

		setup(t, g)
		defer teardown(t, g)

		t.Log("Checking that Srlinux resource doesn't exist in the cluster")
		srlinux := &srlinuxv1.Srlinux{}

		err := k8sClient.Get(ctx, namespacedName, srlinux)

		g.Expect(errors.IsNotFound(err)).To(BeTrue())

		t.Log("Creating the custom resource without parameters for the Kind Srlinux")
		srlinux = &srlinuxv1.Srlinux{
			ObjectMeta: metav1.ObjectMeta{
				Name:      SrlinuxName,
				Namespace: SrlinuxNamespace,
			},
			TypeMeta: metav1.TypeMeta{
				Kind:       "Srlinux",
				APIVersion: "kne.srlinux.dev/v1",
			},
			Spec: srlinuxv1.SrlinuxSpec{
				Config: &srlinuxv1.NodeConfig{
					Image: testImageName,
				},
			},
		}
		g.Expect(k8sClient.Create(ctx, srlinux)).Should(Succeed())

		t.Log("Checking if the custom resource was successfully created")
		g.Eventually(func() error {
			found := &srlinuxv1.Srlinux{}

			return k8sClient.Get(ctx, namespacedName, found)
		}).Should(Succeed())

		// Reconcile is triggered by the creation of the custom resource

		t.Log("Checking if Srlinux Pod was successfully created in the reconciliation")
		g.Eventually(func() error {
			found := &corev1.Pod{}

			return k8sClient.Get(ctx, namespacedName, found)
		}).Should(Succeed())

		t.Log("Ensuring the Srlinux CR Status has been updated with the default image")
		g.Eventually(func() error {
			g.Expect(k8sClient.Get(ctx, namespacedName, srlinux)).Should(Succeed())

			if srlinux.Status.Image != defaultImageName {
				return fmt.Errorf("got Srlinux.Status.Image: %s, want: %s", srlinux.Status.Image, defaultImageName)
			}

			return nil
		}).Should(Succeed())

		t.Log("Ensuring the Srlinux CR pod is running")
		g.Eventually(func() bool {
			found := &corev1.Pod{}

			g.Expect(k8sClient.Get(ctx, namespacedName, found)).Should(Succeed())

			return found.Status.Phase == corev1.PodRunning
		}, srlinuxMaxStartupTime, time.Second).Should(BeTrue())

		t.Log("Deleting the custom resource for the Kind Srlinux")
		g.Expect(k8sClient.Delete(ctx, srlinux)).Should(Succeed())

		t.Log("Checking if the custom resource was successfully deleted")
		g.Eventually(func() error {
			found := &srlinuxv1.Srlinux{}

			return k8sClient.Get(ctx, namespacedName, found)
		}).ShouldNot(Succeed())

		// // Reconcile is triggered by the deletion of the custom resource

		t.Log("Checking if the pod was successfully deleted")
		g.Eventually(func() error {
			found := &corev1.Pod{}

			return k8sClient.Get(ctx, namespacedName, found)
		}).ShouldNot(Succeed())
	})
}
