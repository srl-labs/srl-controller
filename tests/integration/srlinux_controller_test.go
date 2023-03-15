/*
Copyright (c) 2021 Nokia. All rights reserved.


Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

1. Redistributions of source code must retain the above copyright notice, this
   list of conditions and the following disclaimer.

2. Redistributions in binary form must reproduce the above copyright notice,
   this list of conditions and the following disclaimer in the documentation
   and/or other materials provided with the distribution.

3. Neither the name of the copyright holder nor the names of its
   contributors may be used to endorse or promote products derived from
   this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE
FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/

package controllers

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	srlinuxv1 "github.com/srl-labs/srl-controller/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Srlinux controller", func() {
	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		SrlinuxName      = "test-srlinux"
		SrlinuxNamespace = "test"
	)

	Context("Srlinux controller test", func() {
		ctx := context.Background()

		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: SrlinuxNamespace,
			},
		}

		typeNamespaceName := types.NamespacedName{Name: SrlinuxName, Namespace: SrlinuxNamespace}

		BeforeEach(func() {
			By("Creating the Namespace to perform the tests")
			Expect(k8sClient.Create(ctx, namespace)).Should(Succeed())
		})

		AfterEach(func() {
			By("Deleting the Namespace to perform the tests")
			_ = k8sClient.Delete(ctx, namespace)
		})

		It("should successfully reconcile a custom resource for Srlinux created with just an image referenced", func() {
			By("Checking that Srlinux resource doesn't exist in the cluster")

			srlinux := &srlinuxv1.Srlinux{}

			err := k8sClient.Get(ctx, typeNamespaceName, srlinux)

			Expect(errors.IsNotFound(err)).To(BeTrue())

			By("Creating the custom resource for the Kind Srlinux")
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
						Image: "srlinux:latest",
					},
				},
			}
			Expect(k8sClient.Create(ctx, srlinux)).Should(Succeed())

			By("Checking if the custom resource was successfully created")
			Eventually(func() error {
				found := &srlinuxv1.Srlinux{}

				return k8sClient.Get(ctx, typeNamespaceName, found)
			}, 10*time.Second, time.Second).Should(Succeed())

			// Reconcile is triggered by the creation of the custom resource

			By("Checking if Srlinux Pod was successfully created in the reconciliation")
			Eventually(func() error {
				found := &corev1.Pod{}

				return k8sClient.Get(ctx, typeNamespaceName, found)
			}, 10*time.Second, time.Second).Should(Succeed())

			By("Ensuring the Srlinux CR Status has been updated")
			Eventually(func() error {
				Expect(k8sClient.Get(ctx, typeNamespaceName, srlinux)).Should(Succeed())

				if srlinux.Status.Image != "srlinux:latest" {
					return fmt.Errorf("got Srlinux.Status.Image: %s, want: %s", srlinux.Status.Image, "srlinux:latest")
				}

				return nil
			}, 10*time.Second, time.Second).Should(Succeed())

			By("Deleting the custom resource for the Kind Srlinux")
			Expect(k8sClient.Delete(ctx, srlinux)).Should(Succeed())

			By("Checking if the custom resource was successfully deleted")
			Eventually(func() error {
				found := &srlinuxv1.Srlinux{}

				return k8sClient.Get(ctx, typeNamespaceName, found)
			}, 10*time.Second, time.Second).ShouldNot(Succeed())

			// because there are no controllers monitoring built-in resources in the envtest cluster,
			// objects do not get deleted, even if an OwnerReference is set up

			// Reconcile is triggered by the deletion of the custom resource
		})
	})
})
