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

package integration_controllers_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	srlinuxv1 "github.com/srl-labs/srl-controller/api/v1"
	srlctrl "github.com/srl-labs/srl-controller/controllers"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	env        *envtest.Environment
	ctx        context.Context
	cancel     context.CancelFunc
	fakeScheme = runtime.NewScheme()
	k8sClient  client.Client
)

// prepareEnvTest sets up the environment vars used by envtest to create a test environment.
// The function executes only if KUBEBUILDER_ASSETS is not set. KUBEBUILDER_ASSETS is set in the Makefile when called via `make test`.
// When not set, the function sets the KUBEBUILDER_ASSETS to the default location of the envtest binaries so that the tests can run
// without the need to use `make test`.
func prepareEnvTest() {
	if os.Getenv("KUBEBUILDER_ASSETS") == "" {
		// Use the default location of the envtest binaries.
		// note that the k8s version must match the version used in the Makefile.
		assetstPath := filepath.Join("..", "..", "bin", "k8s", "1.25.0-linux-amd64")
		if err := os.Setenv("KUBEBUILDER_ASSETS", assetstPath); err != nil {
			panic(fmt.Sprintf("Failed to set KUBEBUILDER_ASSETS: %v", err))
		}
	}
}

// TestMain is the entry point for the integration tests.
// It sets up the envtest environment and starts the controller manager.
func TestMain(m *testing.M) {
	prepareEnvTest()

	ctx, cancel = context.WithCancel(context.TODO())

	logf.SetLogger(zap.New(zap.UseDevMode(true)))

	env = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	SetDefaultEventuallyPollingInterval(100 * time.Millisecond)
	SetDefaultEventuallyTimeout(10 * time.Second)

	cfg, err := env.Start()
	if err != nil {
		panic(err)
	}

	// add k8s scheme
	if err := clientgoscheme.AddToScheme(fakeScheme); err != nil {
		panic(err)
	}

	// add srlinux scheme
	if err := srlinuxv1.AddToScheme(fakeScheme); err != nil {
		panic(err)
	}

	k8sClient, err = client.New(cfg, client.Options{Scheme: fakeScheme})
	if err != nil {
		panic(err)
	}

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: fakeScheme,
	})
	if err != nil {
		panic(err)
	}

	err = (&srlctrl.SrlinuxReconciler{
		Client: k8sManager.GetClient(),
		Scheme: k8sManager.GetScheme(),
	}).SetupWithManager(k8sManager)
	if err != nil {
		panic(err)
	}

	go func() {
		fmt.Println("Starting the test environment manager")

		if err := k8sManager.Start(ctx); err != nil {
			panic(fmt.Sprintf("Failed to start the test environment manager: %v", err))
		}
	}()
	<-k8sManager.Elected()

	rc := m.Run()

	cancel()

	fmt.Println("Tearing down the test environment")

	err = env.Stop()
	if err != nil {
		panic(err)
	}

	os.Exit(rc)
}
