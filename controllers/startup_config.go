package controllers

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
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

// createStartupConfigVolumesAndMounts creates volume mounts and volumes for srlinux pod
// if the (startup) config file was provided in the spec.
// Volume mounts happens in the /tmp/startup-config directory and not in the /etc/opt/srlinux
// because we need to support renaming operations on config.json, and bind mount paths are not allowing this.
// Hence the temp location, from which the config file is then copied to /etc/opt/srlinux by the kne-entrypoint.sh.
func createStartupConfigVolumesAndMounts(s *srlinuxv1.Srlinux, pod *corev1.Pod, log logr.Logger) {
	// initialize config path and config file variables
	cfgPath := defaultConfigPath
	if p := s.Spec.GetConfig().ConfigPath; p != "" {
		cfgPath = p
	}

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

// handleSrlinuxStartupConfig handles the startup config provisioning.
func (r *SrlinuxReconciler) handleSrlinuxStartupConfig(
	ctx context.Context,
	log logr.Logger,
	update *bool,
	srlinux *srlinuxv1.Srlinux,
) {
	if srlinux.Status.StartupConfig.Phase != "" {
		log.Info("startup config already processed, skipping")

		return
	}

	// we need to wait for podIP to be ready as well as the network to be ready
	// we do this before even checking if the startup config is provided
	// because we need to create a checkpoing in any case
	ip := r.waitPodIPReady(ctx, log, srlinux)

	// even though the SR Linux management server is ready, the network might not be ready yet
	// which results in transport errors when trying to open the scrapligo network driver.
	// Hence we need to wait for the network to be ready.
	driver := r.waitNetworkReady(ctx, log, ip)
	if driver == nil {
		return
	}
	defer driver.Close()

	// if startup config data is not provided and Phase hasn't been set yet, set the Startup Config state to "not-provided"
	// and create a checkpoint
	if !srlinux.Spec.GetConfig().ConfigDataPresent && srlinux.Status.StartupConfig.Phase == "" {
		log.Info("no startup config data provided")

		srlinux.Status.StartupConfig.Phase = "not-provided"
		*update = true

		err := createInitCheckpoint(ctx, driver, log)
		if err != nil {
			log.Error(err, "failed to create initial checkpoint")
		}

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

	err = createInitCheckpoint(ctx, driver, log)
	if err != nil {
		log.Error(err, "failed to create initial checkpoint after loading startup config")
	}
}

// loadStartupConfig loads the provided startup config into the SR Linux device.
// It distinct between CLI- and JSON-styled configs and applies them accordingly.
func loadStartupConfig(
	_ context.Context,
	d *network.Driver,
	fileName string,
	log logr.Logger,
) error {
	cmds := createStartupLoadCmds(fileName, defaultConfigPath)

	r, err := d.SendConfigs(cmds)
	if err != nil {
		log.Error(err, "failed to send commands")

		return err
	}

	if r.Failed != nil {
		log.Error(r.Failed, "applying commands failed")

		return r.Failed
	}

	return nil
}

// createInitCheckpoint creates a checkpoint named "initial".
// This checkpoint is used to reset the device to the initial state, which is the state
// node booted with and (if present) with applied startup config.
func createInitCheckpoint(
	_ context.Context,
	d *network.Driver,
	log logr.Logger,
) error {
	log.Info("Creating initial checkpoint...")

	// sometimes status of srlinux cr is not updated immediately,
	// resulting in several attempts to load configuration and create checkpoint
	// so we need to check if the checkpoint already exists and bail out if so
	checkCheckpointCmd := "info from state system configuration checkpoint *"
	r, err := d.SendCommand(checkCheckpointCmd)
	if err != nil {
		log.Error(err, "failed to send command")

		return err
	}

	if strings.Contains(r.Result, "initial") {
		log.Info("initial checkpoint already exists, skipping")

		return nil
	}

	cmd := "/tools system configuration generate-checkpoint name initial"

	r, err = d.SendCommand(cmd)
	if err != nil {
		log.Error(err, "failed to send command")

		return err
	}

	if r.Failed != nil {
		log.Error(r.Failed, "applying command failed")

		return err
	}

	return nil
}

// createStartupLoadCmds creates the commands to be sent to the device based on the extension of the
// provided startup config. It supports CLI- and JSON-styled configs.
// After loading the configuration it saves it to the startup config.
func createStartupLoadCmds(fileName, path string) []string {
	ext := filepath.Ext(fileName)

	var cmds []string

	switch ext {
	case ".json":
		cmds = append(cmds, fmt.Sprintf("load file %s/%s", path, fileName))
	case ".cli":
		cmds = append(cmds, fmt.Sprintf("source %s/%s", path, fileName))
	}

	cmds = append(cmds, "commit save")

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
				log.Info("pod IP assigned", "pod-ip", ip)

				return ip
			}
		}
	}
}

// getPodIP returns the pod IP.
func (r *SrlinuxReconciler) getPodIP(ctx context.Context, srlinux *srlinuxv1.Srlinux) string {
	pod := &corev1.Pod{}
	_ = r.Get(ctx, types.NamespacedName{Name: srlinux.Name, Namespace: srlinux.Namespace}, pod)

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
			log.Error(fmt.Errorf("timed out waiting network readiness"), "Network not ready") //nolint:goerr113

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
func (*SrlinuxReconciler) getNetworkDriver(_ context.Context, log logr.Logger, podIP string) *network.Driver {
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
