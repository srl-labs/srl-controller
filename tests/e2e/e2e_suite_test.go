// Copyright 2023 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package e2e_controllers_test

import (
	"context"
	"os"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	srlinuxv1 "github.com/srl-labs/srl-controller/api/v1"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	ctx       context.Context
	cancel    context.CancelFunc
	k8sClient client.Client
)

// TestMain is the entry point for the e2e tests.
// It sets up the test environment and initializes a k8s client.
func TestMain(m *testing.M) {
	ctx, cancel = context.WithCancel(context.TODO())

	logf.SetLogger(zap.New(zap.UseDevMode(true)))

	SetDefaultEventuallyPollingInterval(100 * time.Millisecond)
	SetDefaultEventuallyTimeout(10 * time.Second)

	// add srlinux scheme
	if err := srlinuxv1.AddToScheme(scheme.Scheme); err != nil {
		panic(err)
	}

	var err error

	k8sClient, err = client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: scheme.Scheme})
	if err != nil {
		panic(err)
	}

	rc := m.Run()

	cancel()

	os.Exit(rc)
}
