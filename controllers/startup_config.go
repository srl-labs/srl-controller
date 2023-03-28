package controllers

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/go-logr/logr"
	"github.com/scrapli/scrapligo/driver/network"
	"github.com/scrapli/scrapligo/driver/options"
	"github.com/scrapli/scrapligo/platform"
	srlinuxv1 "github.com/srl-labs/srl-controller/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	username = "admin"
	password = "NokiaSrl1!"

	// default path to a startup config directory
	// the default for config file name resides within kne.
	defaultConfigPath = "/tmp/startup-config"

	podIPReadyTimeout  = 60 * time.Second
	podIPReadyInterval = 2 * time.Second
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
				// so we use it as the source for the volume
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
	update *bool,
	srlinux *srlinuxv1.Srlinux,
) {
	if srlinux.Status.StartupConfig.Phase == "loaded" ||
		srlinux.Status.StartupConfig.Phase == "failed" {
		log.Info("startup config load already tried, skipping")

		return
	}

	// if startup config data is not provided and the state is not "not-provided", set the state to "not-provided"
	// so that we only log the message once
	if !srlinux.Spec.GetConfig().ConfigDataPresent {
		log.Info("no startup config data provided")

		srlinux.Status.StartupConfig.Phase = "not-provided"
		*update = true

		return
	}

	log.Info("startup config provided, starting config provisioning...")

	ip := r.waitPodIPReady(ctx, log, srlinux)

	// even though the SR Linux management server is ready, the network might not be ready yet
	// which results in transport errors when trying to open the scrapligo network driver.
	// Hence we need to wait for the network to be ready.
	driver := r.waitNetworkReady(ctx, log, ip)
	if driver == nil {
		return
	}

	log.Info("Loading provided startup configuration...", "filename",
		srlinux.Spec.GetConfig().ConfigFile, "path", defaultConfigPath)

	err := loadStartupConfig(ctx, driver, srlinux.Spec.GetConfig().ConfigFile, log)
	if err != nil {
		srlinux.Status.StartupConfig.Phase = "failed"
		*update = true

		log.Error(err, "failed to load provided startup configuration")

		return
	}

	log.Info("Loaded provided startup configuration...")

	srlinux.Status.StartupConfig.Phase = "loaded"
	*update = true
}

// loadStartupConfig loads the provided startup config into the SR Linux device.
// It distinct between CLI- and JSON-styled configs and applies them accordingly.
func loadStartupConfig(
	_ context.Context,
	d *network.Driver,
	fileName string,
	log logr.Logger,
) error {
	defer d.Close()

	cmds := createCmds(fileName, defaultConfigPath)

	r, err := d.SendConfigs(cmds)
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

// createCmds creates the commands to be sent to the device based on the extension of the
// provided startup config.
func createCmds(fileName, path string) []string {
	ext := filepath.Ext(fileName)

	cmds := []string{}

	switch ext {
	case ".json":
		cmds = append(cmds, fmt.Sprintf("load file %s/%s", path, fileName))
	case ".cli":
		cmds = append(cmds, fmt.Sprintf("source %s/%s", path, fileName))
	}

	cmds = append(cmds, "commit now")

	return cmds
}

// podIPReady checks if the pod IP is assigned and sets the startup config phase to "pending" if not.
func (r *SrlinuxReconciler) waitPodIPReady(
	ctx context.Context,
	log logr.Logger,
	srlinux *srlinuxv1.Srlinux,
) (podIP string) {
	timeout := time.After(podIPReadyTimeout)
	tick := time.NewTicker(podIPReadyInterval)

	defer tick.Stop()

	for {
		select {
		case <-timeout:
			log.Error(fmt.Errorf("timed out waiting for pod IP"), "pod IP not assigned") //nolint:goerr113
			return ""
		case <-tick.C:
			ip := r.getPodIP(ctx, srlinux)
			if ip != "" {
				log.Info("pod IP assigned, provisioning configuration", "pod-ip", ip)
				return ip
			}
		}
	}
}

// getPodIP returns the pod IP.
func (r *SrlinuxReconciler) getPodIP(ctx context.Context, srlinux *srlinuxv1.Srlinux) string {
	pod := &corev1.Pod{}
	r.Get(ctx, types.NamespacedName{Name: srlinux.Name, Namespace: srlinux.Namespace}, pod)

	return pod.Status.PodIP
}

// waitNetworkReady checks if the network driver is ready and returns it.
func (r *SrlinuxReconciler) waitNetworkReady(
	ctx context.Context,
	log logr.Logger,
	podIP string,
) *network.Driver {
	timeout := time.After(podIPReadyTimeout)

	tick := time.NewTicker(podIPReadyInterval)
	defer tick.Stop()

	for {
		select {
		case <-timeout:
			log.Error(fmt.Errorf("timed out waiting network readiness"), "Network not ready")
			return nil
		case <-tick.C:
			log.Info("waiting for network readiness...")

			d := r.getNetworkDriver(ctx, log, podIP)
			if d != nil {
				log.Info("network ready")
				return d
			}
		}
	}
}

// getNetworkDriver returns the opened network driver for a given pod IP.
func (r *SrlinuxReconciler) getNetworkDriver(_ context.Context, log logr.Logger, podIP string) *network.Driver {
	p, err := platform.NewPlatform(
		"nokia_srl",
		podIP,
		options.WithAuthNoStrictKey(),
		options.WithTransportType("standard"),
		options.WithAuthUsername(username),
		options.WithAuthPassword(password),
	)
	if err != nil {
		log.Error(err, "failed to create platform")

		return nil
	}

	d, err := p.GetNetworkDriver()
	if err != nil {
		log.Error(err, "failed to fetch network driver from the platform")

		return nil
	}

	err = d.Open()
	if err != nil {
		log.Error(err, "failed to open driver")

		return nil
	}

	log.Info("SSH connection to pod established")

	return d
}
