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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	srlinuxv1 "github.com/srl-labs/srl-controller/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Srlinux controller", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		SrlinuxName      = "test-srlinux"
		SrlinuxNamespace = "test"

		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
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
			err := k8sClient.Create(ctx, namespace)
			Expect(err).To(Not(HaveOccurred()))
		})

		AfterEach(func() {
			By("Deleting the Namespace to perform the tests")
			_ = k8sClient.Delete(ctx, namespace)

		})

		It("should succesfully reconcile a custom resource for Srlinux", func() {
			By("Creating the custom resource for the Kind Srlinux")

			srlinux := &srlinuxv1.Srlinux{}

			err := k8sClient.Get(ctx, typeNamespaceName, srlinux)

			if err != nil && errors.IsNotFound(err) {

				srlinux := &srlinuxv1.Srlinux{
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
				// Expect(k8sClient.Create(ctx, srlinux)).Should(Succeed())
				err = k8sClient.Create(ctx, srlinux)
				Expect(err).To(Not(HaveOccurred()))
			}

			// srlinuxLookupKey := types.NamespacedName{Name: SrlinuxName, Namespace: SrlinuxNamespace}
			// createdSrlinux := &srlinuxv1.Srlinux{}

			// // We'll need to retry getting this newly created resource, given that creation may not immediately happen.
			// Eventually(func() bool {
			// 	err := k8sClient.Get(ctx, srlinuxLookupKey, createdSrlinux)
			// 	return err == nil
			// }, timeout, interval).Should(BeTrue())

			// // Let's make sure our Schedule string value was properly converted/handled.
			// Expect(createdSrlinux.Spec.Config.Image).Should(Equal("srlinux:latest"))

			By("Checking if the custom resource was successfully created")
			Eventually(func() error {
				found := &srlinuxv1.Srlinux{}
				return k8sClient.Get(ctx, typeNamespaceName, found)
			}, time.Minute, time.Second).Should(Succeed())

			By("Reconciling the custom resource created")
			srlinuxReconciler := &SrlinuxReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err = srlinuxReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespaceName,
			})
			Expect(err).To(Not(HaveOccurred()))
		})
	})
})
