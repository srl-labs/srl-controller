// Copyright 2022 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package controllers

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	srlinuxv1 "github.com/srl-labs/srl-controller/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
)

func TestPodForSrlinux_RebootPersistence(t *testing.T) {
	tests := []struct {
		name              string
		srl               *srlinuxv1.Srlinux
		expectStartupVol  bool
		expectedConfigDir string
	}{
		{
			name: "without startup config",
			srl: &srlinuxv1.Srlinux{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-srl",
					Namespace: "default",
				},
				Spec: srlinuxv1.SrlinuxSpec{
					Config: &srlinuxv1.NodeConfig{
						Image: "ghcr.io/nokia/srlinux:latest",
					},
				},
			},
			expectStartupVol: false,
		},
		{
			name: "with default startup config path",
			srl: &srlinuxv1.Srlinux{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-srl-startup",
					Namespace: "default",
				},
				Spec: srlinuxv1.SrlinuxSpec{
					Config: &srlinuxv1.NodeConfig{
						Image:             "ghcr.io/nokia/srlinux:latest",
						ConfigDataPresent: true,
					},
				},
			},
			expectStartupVol:  true,
			expectedConfigDir: "/tmp/startup-config",
		},
		{
			name: "with custom startup config path",
			srl: &srlinuxv1.Srlinux{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-srl-custom",
					Namespace: "default",
				},
				Spec: srlinuxv1.SrlinuxSpec{
					Config: &srlinuxv1.NodeConfig{
						Image:             "ghcr.io/nokia/srlinux:latest",
						ConfigDataPresent: true,
						ConfigPath:        "/custom/startup/path",
					},
				},
			},
			expectStartupVol:  true,
			expectedConfigDir: "/custom/startup/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			reconciler := &SrlinuxReconciler{
				Scheme: scheme.Scheme,
			}

			pod := reconciler.podForSrlinux(context.Background(), tt.srl)
			g.Expect(pod).NotTo(BeNil())

			// 1. Verify srl-state volume with EmptyDir is always present
			var stateVol *corev1.Volume
			for i := range pod.Spec.Volumes {
				v := &pod.Spec.Volumes[i]
				if v.Name == srlStateVolName {
					stateVol = v
					break
				}
			}
			g.Expect(stateVol).NotTo(BeNil(), "expected srl-state volume to be present in pod.Spec.Volumes")
			g.Expect(stateVol.EmptyDir).NotTo(BeNil(), "expected srl-state volume source to be EmptyDir")

			// 2. Verify srl-state volume mount at /etc/opt/srlinux in container
			g.Expect(pod.Spec.Containers).NotTo(BeEmpty())
			var stateMount *corev1.VolumeMount
			for i := range pod.Spec.Containers[0].VolumeMounts {
				vm := &pod.Spec.Containers[0].VolumeMounts[i]
				if vm.Name == srlStateVolName {
					stateMount = vm
					break
				}
			}
			g.Expect(stateMount).NotTo(BeNil(), "expected srl-state volume mount to be present in container volume mounts")
			g.Expect(stateMount.MountPath).To(Equal(srlStateMntPath))
			g.Expect(stateMount.MountPath).To(Equal("/etc/opt/srlinux"))

			// 3. Verify startup-config volume and mount interaction if expected
			var startupVol *corev1.Volume
			for i := range pod.Spec.Volumes {
				v := &pod.Spec.Volumes[i]
				if v.Name == "startup-config-volume" {
					startupVol = v
					break
				}
			}

			var startupMount *corev1.VolumeMount
			for i := range pod.Spec.Containers[0].VolumeMounts {
				vm := &pod.Spec.Containers[0].VolumeMounts[i]
				if vm.Name == "startup-config-volume" {
					startupMount = vm
					break
				}
			}

			if tt.expectStartupVol {
				g.Expect(startupVol).NotTo(BeNil(), "expected startup-config-volume in pod.Spec.Volumes")
				g.Expect(startupVol.ConfigMap).NotTo(BeNil())
				g.Expect(startupVol.ConfigMap.Name).To(Equal(tt.srl.Name + "-config"))

				g.Expect(startupMount).NotTo(BeNil(), "expected startup-config-volume mount in container")
				g.Expect(startupMount.MountPath).To(Equal(tt.expectedConfigDir))
				// Ensure startup config does not overwrite or shadow the persistent srl-state mount path
				g.Expect(startupMount.MountPath).NotTo(Equal(srlStateMntPath))
			} else {
				g.Expect(startupVol).To(BeNil(), "did not expect startup-config-volume")
				g.Expect(startupMount).To(BeNil(), "did not expect startup-config-volume mount")
			}
		})
	}
}
