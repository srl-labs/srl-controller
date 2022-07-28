/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package controllers contains srlinux k8s/kne controller code
package controllers

import (
	"context"
	"embed"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	typesv1alpha1 "github.com/srl-labs/srl-controller/api/types/v1alpha1"
)

const (
	initContainerName        = "networkop/init-wait:latest"
	variantsVolName          = "variants"
	variantsVolMntPath       = "/tmp/topo"
	variantsTemplateTempName = "topo-template.yml"
	variantsCfgMapName       = "srlinux-variants"

	topomacVolName    = "topomac-script"
	topomacVolMntPath = "/tmp/topomac"
	topomacCfgMapName = "srlinux-topomac-script"

	entrypointVolName       = "kne-entrypoint"
	entrypointVolMntPath    = "/kne-entrypoint.sh"
	entrypointVolMntSubPath = "kne-entrypoint.sh"
	entrypointCfgMapName    = "srlinux-kne-entrypoint"

	// default path to a startup config file
	// the default for config file name resides within kne.
	defaultConfigPath = "/etc/opt/srlinux"

	fileMode777 = 0o777

	srlinuxPodAffinityWeight = 100
)

// VariantsFS is variable without fs assignment, since it is used in main.go
// to assign a value for an fs that is in the outer scope of srlinux_controller.go.
var VariantsFS embed.FS // nolint:gochecknoglobals

// SrlinuxReconciler reconciles a Srlinux object.
type SrlinuxReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=kne.srlinux.dev,resources=srlinuxes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kne.srlinux.dev,resources=srlinuxes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kne.srlinux.dev,resources=srlinuxes/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// Modify the Reconcile function to compare the state specified by
// the Srlinux object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *SrlinuxReconciler) Reconcile(
	ctx context.Context,
	req ctrl.Request,
) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	srlinux := &typesv1alpha1.Srlinux{}

	if res, isReturn, err := r.checkSrlinuxCR(ctx, log, req, srlinux); isReturn {
		return res, err
	}

	// Check if the srlinux pod already exists, if not create a new one
	found := &corev1.Pod{}

	if res, isReturn, err := r.checkSrlinuxPod(ctx, log, srlinux, found); isReturn {
		return res, err
	}

	// updating Srlinux status
	if res, isReturn, err := r.updateSrlinuxStatus(ctx, log, srlinux, found); isReturn {
		return res, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SrlinuxReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&typesv1alpha1.Srlinux{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}

func (r *SrlinuxReconciler) checkSrlinuxCR(
	ctx context.Context,
	log logr.Logger,
	req ctrl.Request,
	srlinux *typesv1alpha1.Srlinux,
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
		log.Error(err, "Failed to get Srlinux")

		return ctrl.Result{}, true, err
	}

	return ctrl.Result{}, false, nil
}

func (r *SrlinuxReconciler) checkSrlinuxPod(
	ctx context.Context,
	log logr.Logger,
	srlinux *typesv1alpha1.Srlinux,
	found *corev1.Pod,
) (ctrl.Result, bool, error) {
	err := r.Get(ctx, types.NamespacedName{Name: srlinux.Name, Namespace: srlinux.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		err = createConfigMaps(ctx, r, srlinux.Namespace, log)
		if err != nil {
			return ctrl.Result{}, true, err
		}
		// Define a new srlinux pod
		pod := r.podForSrlinux(ctx, srlinux)
		log.Info("Creating a new Pod", "Pod.Namespace", pod.Namespace, "Pod.Name", pod.Name)

		err = r.Create(ctx, pod)
		if err != nil {
			log.Error(
				err,
				"Failed to create new Pod",
				"Pod.Namespace",
				pod.Namespace,
				"Pod.Name",
				pod.Name,
			)

			return ctrl.Result{}, true, err
		}

		// Pod created successfully - return and requeue
		return ctrl.Result{Requeue: true}, true, nil
	} else if err != nil {
		log.Error(err, "Failed to get Pod")

		return ctrl.Result{}, true, err
	}

	return ctrl.Result{}, false, err
}

func (r *SrlinuxReconciler) updateSrlinuxStatus(
	ctx context.Context,
	log logr.Logger,
	srlinux *typesv1alpha1.Srlinux,
	found *corev1.Pod,
) (ctrl.Result, bool, error) {
	if !reflect.DeepEqual(found.Spec.Containers[0].Image, srlinux.Status.Image) {
		log.Info("Updating srlinux image status to", "image", found.Spec.Containers[0].Image)
		srlinux.Status.Image = found.Spec.Containers[0].Image

		err := r.Status().Update(ctx, srlinux)
		if err != nil {
			log.Error(err, "Failed to update Srlinux status")

			return ctrl.Result{}, true, err
		}
	}

	return ctrl.Result{}, false, nil
}
