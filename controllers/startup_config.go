package controllers

import (
	"context"

	"github.com/go-logr/logr"
	srlinuxv1 "github.com/srl-labs/srl-controller/api/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *SrlinuxReconciler) handleSrlinuxStartupConfig(
	ctx context.Context,
	log logr.Logger,
	req ctrl.Request,
	srlinux *srlinuxv1.Srlinux,
	pod *corev1.Pod,
) error {
	// if startup config data is not provided and the state is not "not-provided", set the state to "not-provided"
	// so that we only log the message once
	if !srlinux.Spec.GetConfig().ConfigDataPresent {
		if srlinux.Status.StartupConfig.Phase != "not-provided" {
			log.Info("no startup config data provided, continuing")

			srlinux.Status.StartupConfig.Phase = "not-provided"
		}

		return nil
	}

	log.Info("startup config data present", "backing pod IPv4", pod.Status.PodIP)

	return nil
}
