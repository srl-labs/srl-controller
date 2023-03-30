// Copyright 2022 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package controllers

import (
	"context"
	"fmt"

	srlinuxv1 "github.com/srl-labs/srl-controller/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	terminationGracePeriodSeconds = 0
	licensesVolName               = "license"
	licenseFileName               = "license.key"
	licenseMntPath                = "/opt/srlinux/etc/license.key"
	licenseMntSubPath             = "license.key"
	readinessFile                 = "/etc/opt/srlinux/devices/app_ephemeral.mgmt_server.ready_for_config"
	readinessInitialDelay         = 10
	readinessPeriodSeconds        = 5
	readinessFailureThreshold     = 10
)

// podForSrlinux returns a srlinux Pod object.
func (r *SrlinuxReconciler) podForSrlinux(
	ctx context.Context,
	s *srlinuxv1.Srlinux,
) *corev1.Pod {
	log := log.FromContext(ctx)

	if s.Spec.Config.Env == nil {
		s.Spec.Config.Env = map[string]string{}
	}

	s.Spec.Config.Env["SRLINUX"] = "1" // set default srlinux env var

	pod := &corev1.Pod{
		ObjectMeta: createObjectMeta(s),
		Spec: corev1.PodSpec{
			InitContainers:                createInitContainers(s),
			Containers:                    createContainers(s),
			TerminationGracePeriodSeconds: pointer.Int64(terminationGracePeriodSeconds),
			NodeSelector:                  map[string]string{},
			Affinity:                      createAffinity(s),
			Volumes:                       createVolumes(s),
		},
	}

	// handle startup config volume mounts if the startup config was defined
	if s.Spec.Config.ConfigDataPresent {
		createStartupConfigVolumesAndMounts(s, pod, log)
	}

	//nolint:godox
	// TODO: handle the error
	_ = ctrl.SetControllerReference(s, pod, r.Scheme)

	return pod
}

func createObjectMeta(s *srlinuxv1.Srlinux) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      s.Name,
		Namespace: s.Namespace,
		Labels: map[string]string{
			"app":  s.Name,
			"topo": s.Namespace,
		},
	}
}

func createInitContainers(s *srlinuxv1.Srlinux) []corev1.Container {
	return []corev1.Container{{
		Name:  fmt.Sprintf("init-%s", s.Name),
		Image: initContainerName,
		Args: []string{
			fmt.Sprintf("%d", s.Spec.NumInterfaces+1),
			fmt.Sprintf("%d", s.Spec.Config.Sleep),
		},
		ImagePullPolicy: "IfNotPresent",
	}}
}

func createContainers(s *srlinuxv1.Srlinux) []corev1.Container {
	return []corev1.Container{{
		Name:            s.Name,
		Image:           s.Spec.GetImage(),
		Command:         s.Spec.Config.GetCommand(),
		Args:            s.Spec.Config.GetArgs(),
		Env:             toEnvVar(s.Spec.Config.Env),
		Resources:       toResourceRequirements(s.Spec.GetConstraints()),
		ImagePullPolicy: "IfNotPresent",
		SecurityContext: &corev1.SecurityContext{
			Privileged: pointer.Bool(true),
			RunAsUser:  pointer.Int64(0),
		},
		VolumeMounts: createVolumeMounts(s),
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				Exec: &corev1.ExecAction{
					Command: []string{
						"cat",
						readinessFile,
					},
				},
			},
			InitialDelaySeconds: readinessInitialDelay,
			PeriodSeconds:       readinessPeriodSeconds,
			FailureThreshold:    readinessFailureThreshold,
		},
	}}
}

func createAffinity(s *srlinuxv1.Srlinux) *corev1.Affinity {
	return &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				{
					Weight: srlinuxPodAffinityWeight,
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
	}
}

func createVolumes(s *srlinuxv1.Srlinux) []corev1.Volume {
	vols := []corev1.Volume{
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
					DefaultMode: pointer.Int32(fileMode777),
				},
			},
		},
	}

	if s.LicenseKey != "" {
		vols = append(vols, createLicenseVolume(s))
	}

	return vols
}

func createVolumeMounts(s *srlinuxv1.Srlinux) []corev1.VolumeMount {
	vms := []corev1.VolumeMount{
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
	}

	if s.LicenseKey != "" {
		vms = append(vms, createLicenseVolumeMount())
	}

	return vms
}

func createLicenseVolume(s *srlinuxv1.Srlinux) corev1.Volume {
	return corev1.Volume{
		Name: licensesVolName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: srlLicenseSecretName,
				Items: []corev1.KeyToPath{
					{
						Key:  s.LicenseKey,
						Path: licenseFileName,
					},
				},
			},
		},
	}
}

func createLicenseVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      licensesVolName,
		MountPath: licenseMntPath,
		SubPath:   licenseMntSubPath,
	}
}

func toEnvVar(kv map[string]string) []corev1.EnvVar {
	var envVar []corev1.EnvVar
	for k, v := range kv {
		envVar = append(envVar, corev1.EnvVar{
			Name:  k,
			Value: v,
		})
	}
	return envVar
}

func toResourceRequirements(kv map[string]string) corev1.ResourceRequirements {
	r := corev1.ResourceRequirements{
		Requests: map[corev1.ResourceName]resource.Quantity{},
	}
	if v, ok := kv["cpu"]; ok {
		r.Requests["cpu"] = resource.MustParse(v)
	}
	if v, ok := kv["memory"]; ok {
		r.Requests["memory"] = resource.MustParse(v)
	}
	return r
}
