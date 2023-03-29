package controllers

import (
	"context"
	"os"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	srlinuxv1 "github.com/srl-labs/srl-controller/api/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	ctx                 = context.TODO()
	defaultCRName       = "srlinux-test"
	defaultNamespace    = "test"
	namespacedName      = types.NamespacedName{Name: defaultCRName, Namespace: defaultNamespace}
	defaultSrlinuxImage = "srlinux:latest"
)

func TestMain(m *testing.M) {
	err := srlinuxv1.AddToScheme(scheme.Scheme)
	if err != nil {
		panic(err)
	}

	opts := zap.Options{
		Development: true,
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// default timers for gomega Eventually
	SetDefaultEventuallyPollingInterval(100 * time.Millisecond)
	SetDefaultEventuallyTimeout(30 * time.Second)

	os.Exit(m.Run())
}

func TestSrlinuxReconcile(t *testing.T) {
	testsCases := []struct {
		descr      string
		clientObjs []runtime.Object
		// testFn is a test function that is called for a particular test case
		testFn func(t *testing.T, c client.Client, reconciler SrlinuxReconciler, g *GomegaWithT)
	}{
		{
			descr: "valid SR Linux CR exists",
			clientObjs: []runtime.Object{
				&srlinuxv1.Srlinux{
					ObjectMeta: ctrl.ObjectMeta{
						Name:      defaultCRName,
						Namespace: defaultNamespace,
					},
					Spec: srlinuxv1.SrlinuxSpec{
						Config: &srlinuxv1.NodeConfig{
							Image: defaultSrlinuxImage,
						},
					},
				},
			},
			testFn: testReconcileForBasicSrlCR,
		},
		{
			descr:      "SR Linux CR doesn't exists (e.g. deleted)",
			clientObjs: []runtime.Object{},
			testFn:     testReconcileForDeletedCR,
		},
	}

	for _, tc := range testsCases {
		t.Run(tc.descr, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().WithRuntimeObjects(tc.clientObjs...).Build()

			g := NewWithT(t)
			reconciler := SrlinuxReconciler{
				Scheme: scheme.Scheme,
				Client: fakeClient,
			}

			tc.testFn(t, fakeClient, reconciler, g)
		})
	}
}

func testReconcileForBasicSrlCR(_ *testing.T, c client.Client, reconciler SrlinuxReconciler, g *GomegaWithT) {
	g.Eventually(func() bool {
		res, err := reconciler.Reconcile(context.TODO(), reconcile.Request{
			NamespacedName: namespacedName,
		})

		return res.IsZero() && err == nil
	}, 10*time.Second, time.Second).Should(BeTrue())

	// check if the Pod for Srlinux CR has been created
	pod := &corev1.Pod{}
	g.Expect(c.Get(ctx, namespacedName, pod)).To(Succeed())

	// check if CR status is updated
	srlinux := &srlinuxv1.Srlinux{}
	g.Expect(c.Get(ctx, namespacedName, srlinux)).To(Succeed())
	g.Expect(srlinux.Status.Image).To(Equal(defaultSrlinuxImage))
}

func testReconcileForDeletedCR(_ *testing.T, c client.Client, reconciler SrlinuxReconciler, g *GomegaWithT) {
	g.Eventually(func() bool {
		res, err := reconciler.Reconcile(context.TODO(), reconcile.Request{
			NamespacedName: namespacedName,
		})

		return res.IsZero() && err == nil
	}, 10*time.Second, time.Second).Should(BeTrue())

	// check if CR hasn't been created by reconciliation loop
	srlinux := &srlinuxv1.Srlinux{}
	g.Expect(c.Get(ctx, namespacedName, srlinux)).ToNot(Succeed())

	// check if CR hasn't been created
	pod := &corev1.Pod{}
	g.Expect(c.Get(ctx, namespacedName, pod)).ToNot(Succeed())
}
