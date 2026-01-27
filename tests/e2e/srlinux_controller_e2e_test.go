// Copyright 2023 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package e2e_controllers_test

import (
	"fmt"
	"os"
	"os/exec"
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
	SrlinuxNamespace = "test"
	SrlinuxName      = "test-srlinux"
	testImageName    = "ghcr.io/nokia/srlinux:latest"
	defaultImageName = "ghcr.io/nokia/srlinux:latest"
	// time to wait for the Srlinux pod to be ready.
	// 60s looks like a lot, but this is to ensure that slow CI systems have enough time.
	srlinuxMaxReadyTime   = 180 * time.Second
	srlinuxMaxStartupTime = 3 * time.Minute
)

var namespacedName = types.NamespacedName{Name: SrlinuxName, Namespace: SrlinuxNamespace}

var srlTypeMeta = metav1.TypeMeta{
	Kind:       "Srlinux",
	APIVersion: "kne.srlinux.dev/v1",
}

func createNamespace(t *testing.T, g *WithT, namespace *corev1.Namespace) {
	t.Helper()

	t.Log("Creating the namespace")
	g.Expect(k8sClient.Create(ctx, namespace)).Should(Succeed())
}

func deleteNamespace(t *testing.T, _ *WithT, namespace *corev1.Namespace) {
	t.Helper()

	t.Log("Deleting the namespace")

	_ = k8sClient.Delete(ctx, namespace)
}

func createConfigMapFromFile(t *testing.T, g *WithT, name, key, file string) {
	t.Helper()

	t.Logf("Creating the config map name %s, key %s", name, key)

	// read file content from configs directory
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: SrlinuxNamespace,
		},
		Data: map[string]string{
			key: string(b),
		},
	}

	g.Expect(k8sClient.Create(ctx, configMap)).Should(Succeed())
}

// TestSrlinuxReconciler_BareSrlinuxCR tests the reconciliation of the Srlinux custom resource
// which has a bare minimal spec - just the test image.
func TestSrlinuxReconciler_BareSrlinuxCR(t *testing.T) {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: SrlinuxNamespace,
		},
	}

	t.Run("Should reconcile a Srlinux custom resource", func(t *testing.T) {
		g := NewWithT(t)

		createNamespace(t, g, namespace)
		defer deleteNamespace(t, g, namespace)

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

		t.Log("Ensuring the Srlinux CR Ready status reached true")

		g.Eventually(func() bool {
			srl := &srlinuxv1.Srlinux{}
			g.Expect(k8sClient.Get(ctx, namespacedName, srl)).Should(Succeed())

			return srl.Status.Ready == true
		}, srlinuxMaxReadyTime).Should(BeTrue())

		t.Log("Deleting the custom resource for the Kind Srlinux")
		g.Expect(k8sClient.Delete(ctx, srlinux)).Should(Succeed())

		t.Log("Checking if the custom resource was successfully deleted")
		g.Eventually(func() error {
			found := &srlinuxv1.Srlinux{}

			return k8sClient.Get(ctx, namespacedName, found)
		}).ShouldNot(Succeed())

		// Reconcile is triggered by the deletion of the custom resource

		t.Log("Checking if the pod was successfully deleted")
		g.Eventually(func() error {
			found := &corev1.Pod{}

			return k8sClient.Get(ctx, namespacedName, found)
		}).ShouldNot(Succeed())
	})
}

// TestSrlinuxReconciler_WithJSONStartupConfig tests the reconciliation of the Srlinux custom resource
// provided with the JSON-styled startup config.
func TestSrlinuxReconciler_WithJSONStartupConfig(t *testing.T) {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: SrlinuxNamespace,
		},
	}

	srlName := "srl1"

	srlNsName := types.NamespacedName{Name: srlName, Namespace: SrlinuxNamespace}

	setup := func(t *testing.T, g *WithT) {
		t.Helper()

		createNamespace(t, g, namespace)
		createConfigMapFromFile(t, g, srlName+"-config", "config.json", "./configs/test.json")
	}

	t.Run("Should reconcile a Srlinux custom resource", func(t *testing.T) {
		testReconciliationWithConfig(t, setup, srlNsName, "config.json")
	})
}

// TestSrlinuxReconciler_WithJSONStartupConfig tests the reconciliation of the Srlinux custom resource
// provided with the CLI-styled startup config.
func TestSrlinuxReconciler_WithCLIStartupConfig(t *testing.T) {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: SrlinuxNamespace,
		},
	}

	srlName := "srl1"

	srlNsName := types.NamespacedName{Name: srlName, Namespace: SrlinuxNamespace}

	setup := func(t *testing.T, g *WithT) {
		t.Helper()

		createNamespace(t, g, namespace)
		createConfigMapFromFile(t, g, srlName+"-config", "config.cli", "./configs/test.cli")
	}

	t.Run("Should reconcile a Srlinux custom resource", func(t *testing.T) {
		testReconciliationWithConfig(t, setup, srlNsName, "config.cli")
	})
}

func testReconciliationWithConfig(
	t *testing.T,
	setup func(t *testing.T, g *WithT),
	srlNsName types.NamespacedName,
	configFile string,
) {
	g := NewWithT(t)

	setup(t, g)

	t.Log("Checking that Srlinux resources do not exist in the cluster")

	srl := &srlinuxv1.Srlinux{}

	err := k8sClient.Get(ctx, srlNsName, srl)
	g.Expect(errors.IsNotFound(err)).To(BeTrue())

	t.Log("Creating the custom resources with startup config present")

	srl = &srlinuxv1.Srlinux{
		ObjectMeta: metav1.ObjectMeta{
			Name:      srlNsName.Name,
			Namespace: SrlinuxNamespace,
		},
		TypeMeta: srlTypeMeta,
		Spec: srlinuxv1.SrlinuxSpec{
			Config: &srlinuxv1.NodeConfig{
				Image:             testImageName,
				ConfigDataPresent: true,
				ConfigFile:        configFile,
			},
		},
	}
	g.Expect(k8sClient.Create(ctx, srl)).Should(Succeed())

	t.Log("Checking if the custom resources were successfully created")

	g.Eventually(func() error {
		found := &srlinuxv1.Srlinux{}

		return k8sClient.Get(ctx, srlNsName, found)
	}).Should(Succeed())

	// Reconcile is triggered by the creation of the custom resource

	t.Log("Checking if Srlinux Pods were successfully created in the reconciliation")
	g.Eventually(func() error {
		found := &corev1.Pod{}

		return k8sClient.Get(ctx, srlNsName, found)
	}).Should(Succeed())

	t.Log("Ensuring the Srlinux CR Ready status reached true")
	g.Eventually(func() bool {
		srl := &srlinuxv1.Srlinux{}
		g.Expect(k8sClient.Get(ctx, srlNsName, srl)).Should(Succeed())

		return srl.Status.Ready == true
	}, srlinuxMaxReadyTime).Should(BeTrue())

	t.Log("Ensuring Srlinux config state is loaded")
	// reuse max ready time, which should be more than enought to apply config
	g.Eventually(func() bool {
		srl := &srlinuxv1.Srlinux{}
		g.Expect(k8sClient.Get(ctx, srlNsName, srl)).Should(Succeed())

		return srl.Status.StartupConfig.Phase == "loaded"
	}, srlinuxMaxReadyTime).Should(BeTrue())

	t.Log("Ensuring Srlinux config state is applied")
	//nolint:gosec
	cmd := exec.Command("kubectl", "exec", "-n", SrlinuxNamespace,
		srlNsName.Name, "--", "sr_cli", "info", "from", "state", "interface", "mgmt0", "description")

	b, err := cmd.CombinedOutput()

	g.Expect(err).ShouldNot(HaveOccurred())

	g.Expect(string(b)).Should(ContainSubstring("set from e2e test"))
}

// TestSrlinuxReconciler_WithCustomInitImage tests the reconciliation of the Srlinux custom resource
// with a custom init image specified.
func TestSrlinuxReconciler_WithCustomInitImage(t *testing.T) {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: SrlinuxNamespace,
		},
	}

	customInitImage := "us-west1-docker.pkg.dev/kne-external/kne/init-wait:ga"

	t.Run("Should reconcile a Srlinux custom resource with custom init image", func(t *testing.T) {
		g := NewWithT(t)

		createNamespace(t, g, namespace)
		defer deleteNamespace(t, g, namespace)

		t.Log("Checking that Srlinux resource doesn't exist in the cluster")
		srlinux := &srlinuxv1.Srlinux{}

		err := k8sClient.Get(ctx, namespacedName, srlinux)

		g.Expect(errors.IsNotFound(err)).To(BeTrue())

		t.Log("Creating the custom resource with custom init image for the Kind Srlinux")
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
					Image:     testImageName,
					InitImage: customInitImage,
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

		t.Log("Ensuring the custom init image is used in the pod")
		g.Eventually(func() error {
			pod := &corev1.Pod{}
			g.Expect(k8sClient.Get(ctx, namespacedName, pod)).Should(Succeed())

			if len(pod.Spec.InitContainers) == 0 {
				return fmt.Errorf("no init containers found in pod")
			}

			initContainer := pod.Spec.InitContainers[0]
			if initContainer.Image != customInitImage {
				return fmt.Errorf("got init container image: %s, want: %s", initContainer.Image, customInitImage)
			}

			return nil
		}).Should(Succeed())

		t.Log("Ensuring the Srlinux CR pod is running")
		g.Eventually(func() bool {
			found := &corev1.Pod{}

			g.Expect(k8sClient.Get(ctx, namespacedName, found)).Should(Succeed())

			return found.Status.Phase == corev1.PodRunning
		}, srlinuxMaxStartupTime, time.Second).Should(BeTrue())

		t.Log("Ensuring the Srlinux CR Ready status reached true")

		g.Eventually(func() bool {
			srl := &srlinuxv1.Srlinux{}
			g.Expect(k8sClient.Get(ctx, namespacedName, srl)).Should(Succeed())

			return srl.Status.Ready == true
		}, srlinuxMaxReadyTime).Should(BeTrue())

		t.Log("Deleting the custom resource for the Kind Srlinux")
		g.Expect(k8sClient.Delete(ctx, srlinux)).Should(Succeed())

		t.Log("Checking if the custom resource was successfully deleted")
		g.Eventually(func() error {
			found := &srlinuxv1.Srlinux{}

			return k8sClient.Get(ctx, namespacedName, found)
		}).ShouldNot(Succeed())

		// Reconcile is triggered by the deletion of the custom resource

		t.Log("Checking if the pod was successfully deleted")
		g.Eventually(func() error {
			found := &corev1.Pod{}

			return k8sClient.Get(ctx, namespacedName, found)
		}).ShouldNot(Succeed())
	})
}
