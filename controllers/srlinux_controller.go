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
	"embed"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	srlinuxv1 "github.com/srl-labs/srl-controller/api/v1"
)

const (
	controllerNamespace = "srlinux-controller"

	initContainerName        = "ghcr.io/srl-labs/init-wait:latest"
	variantsVolName          = "variants"
	variantsVolMntPath       = "/tmp/topo"
	variantsTemplateTempName = "topo-template.yml"
	variantsCfgMapName       = "srlinux-variants"

	topomacVolName    = "topomac-script"
	topomacVolMntPath = "/tmp/topomac"
	topomacCfgMapName = "srlinux-topomac-script"

	entrypointVolName    = "kne-entrypoint"
	entrypointVolMntPath = "/kne-entrypoint.sh"
	// used to enable mounting a file in an existing folder
	// https://stackoverflow.com/questions/33415913/whats-the-best-way-to-share-mount-one-file-into-a-pod
	entrypointVolMntSubPath = "kne-entrypoint.sh"
	entrypointCfgMapName    = "srlinux-kne-entrypoint"

	fileMode777 = 0o777

	srlinuxPodAffinityWeight = 100
)

//go:embed manifests/variants/*
var variantsFS embed.FS

// SrlinuxReconciler reconciles a Srlinux object.
type SrlinuxReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kne.srlinux.dev,resources=srlinuxes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kne.srlinux.dev,resources=srlinuxes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kne.srlinux.dev,resources=srlinuxes/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// Modify the Reconcile function to compare the state specified by
// the Srlinux object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.1/pkg/reconcile
func (r *SrlinuxReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// track if update is required
	update := false

	srlinux := &srlinuxv1.Srlinux{}

	// isReturn is used to indicate if the called function should return or continue reconciliation.
	// This is needed since empty ctlr.Result{} can't be used to identify if we should return from the reconciliation
	// or continue with the process when using called functions.
	if res, isReturn, err := r.handleSrlinuxCR(ctx, log, req, srlinux); isReturn {
		return res, err
	}

	// Check if the srlinux pod already exists, if not create a new one
	pod := &corev1.Pod{}

	if res, isReturn, err := r.handleSrlinuxPod(ctx, log, &update, srlinux, pod); isReturn {
		return res, err
	}

	// Update the srlinux status after pod creation/handling
	if update {
		if res, isReturn, err := r.updateSrlinuxStatus(ctx, log, req, srlinux); isReturn {
			return res, err
		}
	}

	// SR Linux becomes ready when its Pod readiness probe succeeds.
	// The readiness probe checks if mgmt server is ready to accept config
	if !srlinux.Status.Ready {
		log.Info("SR Linux management server is not yet ready, requeing...")

		// wait 2 sec before requeuing as constant polling is not needed
		return ctrl.Result{}, nil
	}

	r.handleSrlinuxStartupConfig(ctx, log, &update, srlinux)

	// updating Srlinux status
	if update {
		if res, isReturn, err := r.updateSrlinuxStatus(ctx, log, req, srlinux); isReturn {
			return res, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SrlinuxReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&srlinuxv1.Srlinux{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}

// handleSrlinuxCR handles SR Linux custom resource. It fetches the CR based on the namespaced name
// and handles cases when the object appears to be deleted.
func (r *SrlinuxReconciler) handleSrlinuxCR(
	ctx context.Context,
	log logr.Logger,
	req ctrl.Request,
	srlinux *srlinuxv1.Srlinux,
) (ctrl.Result, bool, error) {
	if err := r.Get(ctx, req.NamespacedName, srlinux); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("Srlinux resource not found. Ignoring since object must be deleted",
				"NamespacedName", req.NamespacedName)

			return ctrl.Result{}, true, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "failed to get Srlinux")

		return ctrl.Result{}, true, err
	}

	// successfully got the Srlinux CR, continue with reconciliation
	return ctrl.Result{}, false, nil
}

// handleSrlinuxPod handles Pod lifecycle for Srlinux CR.
func (r *SrlinuxReconciler) handleSrlinuxPod(
	ctx context.Context,
	log logr.Logger,
	update *bool,
	srlinux *srlinuxv1.Srlinux,
	pod *corev1.Pod,
) (ctrl.Result, bool, error) {
	err := r.Get(ctx, types.NamespacedName{Name: srlinux.Name, Namespace: srlinux.Namespace}, pod)
	// if pod was not found, create a new one
	if err != nil && errors.IsNotFound(err) {
		err = createConfigMaps(ctx, r, srlinux, log)
		if err != nil {
			return ctrl.Result{}, true, err
		}

		err = r.createSecrets(ctx, srlinux, log)
		if err != nil {
			return ctrl.Result{}, true, err
		}

		// Define a new srlinux pod
		pod := r.podForSrlinux(ctx, srlinux)

		log.Info("creating a new pod")

		err = r.Create(ctx, pod)
		if err != nil {
			log.Error(
				err, "failed to create new Pod",
			)

			return ctrl.Result{}, true, err
		}

		// Pod created successfully - return and requeue
		return ctrl.Result{Requeue: true}, true, nil
	} else if err != nil {
		log.Error(err, "failed to get Pod")

		return ctrl.Result{}, true, err
	}

	// setting status of srlinux CR
	if srlinux.Status.Image != pod.Spec.Containers[0].Image {
		*update = true
		srlinux.Status.Image = pod.Spec.Containers[0].Image
	}

	if srlinux.Status.Status != string(pod.Status.Phase) {
		*update = true
		srlinux.Status.Status = string(pod.Status.Phase)
	}

	if len(pod.Status.ContainerStatuses) > 0 {
		if srlinux.Status.Ready != pod.Status.ContainerStatuses[0].Ready {
			*update = true
			srlinux.Status.Ready = pod.Status.ContainerStatuses[0].Ready
		}
	}

	return ctrl.Result{}, false, err
}

// updateSrlinuxStatus updates Srlinux status.
func (r *SrlinuxReconciler) updateSrlinuxStatus(
	ctx context.Context,
	log logr.Logger,
	_ ctrl.Request,
	srlinux *srlinuxv1.Srlinux,
) (ctrl.Result, bool, error) {
	log.Info("updating srlinux status", "srlinux-status", srlinux.Status)

	err := r.Status().Update(ctx, srlinux)
	if err != nil {
		log.Error(err, "failed to update Srlinux status")

		return ctrl.Result{}, true, err
	}

	return ctrl.Result{}, false, nil
}
