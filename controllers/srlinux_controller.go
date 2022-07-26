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

package controllers

import (
	"context"
	"embed"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	knenode "github.com/openconfig/kne/topo/node"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	typesv1alpha1 "github.com/srl-labs/srl-controller/api/types/v1alpha1"
)

const (
	InitContainerName        = "networkop/init-wait:latest"
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
)

var VariantsFS embed.FS

// SrlinuxReconciler reconciles a Srlinux object.
type SrlinuxReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kne.srlinux.dev,resources=srlinuxes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kne.srlinux.dev,resources=srlinuxes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kne.srlinux.dev,resources=srlinuxes/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// Modify the Reconcile function to compare the state specified by
// the Srlinux object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *SrlinuxReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	srlinux := &typesv1alpha1.Srlinux{}
	var err error
	if err := r.Get(ctx, req.NamespacedName, srlinux); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("Srlinux resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get Srlinux")
		return ctrl.Result{}, err
	}

	// Check if the srlinux pod already exists, if not create a new one
	found := &corev1.Pod{}
	err = r.Get(ctx, types.NamespacedName{Name: srlinux.Name, Namespace: srlinux.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		err = createConfigMapsIfNeeded(ctx, r, srlinux.Namespace, log)
		if err != nil {
			return ctrl.Result{}, err
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
			return ctrl.Result{}, err
		}
		// Pod created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Pod")
		return ctrl.Result{}, err
	}

	// updating Srlinux status
	if !reflect.DeepEqual(found.Spec.Containers[0].Image, srlinux.Status.Image) {
		log.Info("Updating srlinux image status to", "image", found.Spec.Containers[0].Image)
		srlinux.Status.Image = found.Spec.Containers[0].Image
		err = r.Status().Update(ctx, srlinux)
		if err != nil {
			log.Error(err, "Failed to update Srlinux status")
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

// podForSrlinux returns a srlinux Pod object.
func (r *SrlinuxReconciler) podForSrlinux(
	ctx context.Context,
	s *typesv1alpha1.Srlinux,
) *corev1.Pod {
	log := log.FromContext(ctx)

	if s.Spec.Config.Env == nil {
		s.Spec.Config.Env = map[string]string{}
	}
	s.Spec.Config.Env["SRLINUX"] = "1" // set default srlinux env var

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Name,
			Namespace: s.Namespace,
			Labels: map[string]string{
				"app":  s.Name,
				"topo": s.Namespace,
			},
		},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Name:  fmt.Sprintf("init-%s", s.Name),
				Image: InitContainerName,
				Args: []string{
					fmt.Sprintf("%d", s.Spec.NumInterfaces+1),
					fmt.Sprintf("%d", s.Spec.Config.Sleep),
				},
				ImagePullPolicy: "IfNotPresent",
			}},
			Containers: []corev1.Container{{
				Name:            s.Name,
				Image:           s.Spec.GetImage(),
				Command:         s.Spec.Config.GetCommand(),
				Args:            s.Spec.Config.GetArgs(),
				Env:             knenode.ToEnvVar(s.Spec.Config.Env),
				Resources:       knenode.ToResourceRequirements(s.Spec.GetConstraints()),
				ImagePullPolicy: "IfNotPresent",
				SecurityContext: &corev1.SecurityContext{
					Privileged: pointer.Bool(true),
					RunAsUser:  pointer.Int64(0),
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      variantsVolName,
						MountPath: variantsVolMntPath,
					},
					{
						Name:      topomacVolName,
						MountPath: topomacVolMntPath,
					},
					{
						Name:      entrypointVolName,
						MountPath: entrypointVolMntPath,
						SubPath:   entrypointVolMntSubPath,
					},
				},
			}},
			TerminationGracePeriodSeconds: pointer.Int64(0),
			NodeSelector:                  map[string]string{},
			Affinity: &corev1.Affinity{
				PodAntiAffinity: &corev1.PodAntiAffinity{
					PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
						{
							Weight: 100,
							PodAffinityTerm: corev1.PodAffinityTerm{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{{
										Key:      "topo",
										Operator: "In",
										Values:   []string{s.Name},
									}},
								},
								TopologyKey: "kubernetes.io/hostname",
							},
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: variantsVolName,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: variantsCfgMapName,
							},
							Items: []corev1.KeyToPath{
								{
									Key:  s.Spec.GetModel(),
									Path: variantsTemplateTempName,
								},
							},
						},
					},
				},
				{
					Name: topomacVolName,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: topomacCfgMapName,
							},
						},
					},
				},
				{
					Name: entrypointVolName,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: entrypointCfgMapName,
							},
							DefaultMode: pointer.Int32(0777),
						},
					},
				},
			},
		},
	}

	// initialize config path and config file variables
	cfgPath := defaultConfigPath
	if p := s.Spec.GetConfig().ConfigPath; p != "" {
		cfgPath = p
	}

	cfgFile := s.Spec.GetConfig().ConfigFile

	// only create startup config mounts if the config data was set in kne
	if s.Spec.Config.ConfigDataPresent {
		log.Info(
			"Adding volume for startup config to pod spec",
			"volume.name",
			"startup-config-volume",
			"mount.path",
			cfgPath+"/"+cfgFile,
		)
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: "startup-config-volume",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: fmt.Sprintf("%s-config", s.Name),
					},
				},
			},
		})

		pod.Spec.Containers[0].VolumeMounts = append(
			pod.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      "startup-config-volume",
				MountPath: cfgPath + "/" + cfgFile,
				SubPath:   cfgFile,
				ReadOnly:  true,
			},
		)
	}

	_ = ctrl.SetControllerReference(s, pod, r.Scheme)

	return pod
}

// createConfigMapsIfNeeded creates srlinux-variants and srlinux-topomac config maps which every srlinux pod needs to mount.
func createConfigMapsIfNeeded(
	ctx context.Context,
	r *SrlinuxReconciler,
	ns string,
	log logr.Logger,
) error {
	// Check if the variants cfg map already exists, if not create a new one
	cfgMap := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{Name: variantsCfgMapName, Namespace: ns}, cfgMap)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating a new variants configmap")
		data, err := VariantsFS.ReadFile("manifests/variants/srl_variants.yml")
		if err != nil {
			return err
		}
		decoder := serializer.NewCodecFactory(clientgoscheme.Scheme).UniversalDecoder()
		err = runtime.DecodeInto(decoder, data, cfgMap)
		if err != nil {
			return err
		}
		cfgMap.ObjectMeta.Namespace = ns
		err = r.Create(ctx, cfgMap)
		if err != nil {
			return err
		}
	}

	// Check if the topomac script cfg map already exists, if not create a new one
	cfgMap = &corev1.ConfigMap{}
	err = r.Get(ctx, types.NamespacedName{Name: topomacCfgMapName, Namespace: ns}, cfgMap)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating a new topomac script configmap")
		data, err := VariantsFS.ReadFile("manifests/variants/topomac.yml")
		if err != nil {
			return err
		}
		decoder := serializer.NewCodecFactory(clientgoscheme.Scheme).UniversalDecoder()
		err = runtime.DecodeInto(decoder, data, cfgMap)
		if err != nil {
			return err
		}
		cfgMap.ObjectMeta.Namespace = ns
		err = r.Create(ctx, cfgMap)
		if err != nil {
			return err
		}
	}

	// Check if the kne-entrypoint cfg map already exists, if not create a new one
	cfgMap = &corev1.ConfigMap{}
	err = r.Get(ctx, types.NamespacedName{Name: entrypointCfgMapName, Namespace: ns}, cfgMap)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating a new kne-entrypoint configmap")
		data, err := VariantsFS.ReadFile("manifests/variants/kne-entrypoint.yml")
		if err != nil {
			return err
		}
		decoder := serializer.NewCodecFactory(clientgoscheme.Scheme).UniversalDecoder()
		err = runtime.DecodeInto(decoder, data, cfgMap)
		if err != nil {
			return err
		}
		cfgMap.ObjectMeta.Namespace = ns
		err = r.Create(ctx, cfgMap)
		if err != nil {
			return err
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SrlinuxReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&typesv1alpha1.Srlinux{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}
