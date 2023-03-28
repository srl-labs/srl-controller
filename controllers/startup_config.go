package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/scrapli/scrapligo/driver/options"
	"github.com/scrapli/scrapligo/platform"
	srlinuxv1 "github.com/srl-labs/srl-controller/api/v1"
	corev1 "k8s.io/api/core/v1"
)

const (
	username = "admin"
	password = "NokiaSrl1!"

	// default path to a startup config directory
	// the default for config file name resides within kne.
	defaultConfigPath = "/tmp/startup-config"
)

// handleStartupConfig creates volume mounts and volumes for srlinux pod
// if the (startup) config file was provided in the spec.
// Volume mounts happens in the /tmp/startup-config directory and not in the /etc/opt/srlinux
// because we need to support renaming operations on config.json, and bind mount paths are not allowing this.
// Hence the temp location, from which the config file is then copied to /etc/opt/srlinux by the kne-entrypoint.sh.
func handleStartupConfig(s *srlinuxv1.Srlinux, pod *corev1.Pod, log logr.Logger) {
	// initialize config path and config file variables
	cfgPath := defaultConfigPath
	if p := s.Spec.GetConfig().ConfigPath; p != "" {
		cfgPath = p
	}

	// only create startup config mounts if the config data was set in kne
	if s.Spec.Config.ConfigDataPresent {
		log.Info(
			"Adding volume for startup config to pod spec",
			"volume.name",
			"startup-config-volume",
			"mount.path",
			cfgPath,
		)

		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: "startup-config-volume",
			VolumeSource: corev1.VolumeSource{
				// kne creates the configmap with the name <node-name>-config,
				// so we use it as the source for the volume mount
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
				MountPath: cfgPath,
				ReadOnly:  false,
			},
		)
	}
}

func (r *SrlinuxReconciler) handleSrlinuxStartupConfig(
	ctx context.Context,
	log logr.Logger,
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

	log.Info("startup config data present")

	if pod.Status.PodIP == "" {
		log.Info("pod IP not yet assigned, skipping configuration provisioning")

		srlinux.Status.StartupConfig.Phase = "pending"

		return nil
	}

	log.Info("pod IP assigned, provisioning configuration", "pod-ip", pod.Status.PodIP)

	return loadStartupConfig(ctx, pod.Status.PodIP, log)
}

func loadStartupConfig(
	ctx context.Context,
	podIP string,
	log logr.Logger,
) error {
	p, err := platform.NewPlatform(
		// cisco_iosxe refers to the included cisco iosxe platform definition
		"nokia_srl",
		podIP,
		options.WithAuthNoStrictKey(),
		options.WithTransportType("standard"),
		options.WithAuthUsername(username),
		options.WithAuthPassword(password),
	)
	if err != nil {
		log.Error(err, "failed to create platform")

		return err
	}

	d, err := p.GetNetworkDriver()
	if err != nil {
		log.Error(err, "failed to fetch network driver from the platform")

		return err
	}

	err = d.Open()
	if err != nil {
		log.Error(err, "failed to open driver")

		return err
	}

	defer d.Close()

	r, err := d.SendConfigs([]string{"source /tmp/startup-config/config.json", "commit now"})
	if err != nil {
		log.Error(err, "failed to send commands")

		return err
	}

	if r.Failed != nil {
		log.Error(r.Failed, "applying commands failed")

		return err
	}

	return nil
}
