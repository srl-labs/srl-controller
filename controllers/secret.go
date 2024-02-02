// Copyright 2022 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package controllers

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	srlinuxv1 "github.com/srl-labs/srl-controller/api/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var ErrLicenseProvisioning = errors.New("license provisioning failed")

// createSecrets creates secrets such as srlinux-licenses.
func (r *SrlinuxReconciler) createSecrets(
	ctx context.Context,
	s *srlinuxv1.Srlinux,
	log logr.Logger,
) error {
	secret, err := r.addOrUpdateLicenseSecret(ctx, s, log)
	if err != nil {
		return err
	}

	v := s.Spec.GetImageVersion()

	if v.Major == "0" {
		log.Info(
			"SR Linux image version could not be parsed, will continue without handling license",
		)

		return nil
	}

	log.Info("SR Linux image version parsed", "version", v)

	// set license key matching image version
	s.InitLicenseKey(ctx, secret, v)

	return nil
}

func (r *SrlinuxReconciler) addOrUpdateLicenseSecret(
	ctx context.Context,
	s *srlinuxv1.Srlinux,
	log logr.Logger,
) (*corev1.Secret, error) {
	secret := &corev1.Secret{}

	// if secret is already present in the s.Namespace, we need to update it
	if err := r.Get(ctx, types.NamespacedName{
		Name:      srlLicenseSecretName,
		Namespace: s.Namespace,
	}, secret); err == nil {
		return r.updateLicenseSecret(ctx, s, log, secret)
	}

	// otherwise we need to copy a secret from controller's namespace
	return r.copyLicenseSecret(ctx, s, log)
}

// copyLicenseSecret finds a secret with srlinux licenses in srlinux-controller namespace
// and copies it to the srlinux CR namespace and returns the pointer to the newly created secret.
// If original secret doesn't exist (i.e. when no licenses were provisioned by a user)
// then nothing gets copied and nil returned.
func (r *SrlinuxReconciler) copyLicenseSecret(
	ctx context.Context,
	s *srlinuxv1.Srlinux,
	log logr.Logger,
) (*corev1.Secret, error) {
	secret := &corev1.Secret{}

	// find license secret in controller' ns to copy from
	// return silently if not found.
	err := r.Get(
		ctx,
		types.NamespacedName{Name: srlLicenseSecretName, Namespace: controllerNamespace},
		secret,
	)
	if err != nil && k8serrors.IsNotFound(err) {
		log.Info(
			"secret with licenses is not found in controller's namespace, skipping copy to lab namespace",
			"secret name",
			srlLicenseSecretName,
			"controller namespace",
			controllerNamespace,
		)

		return nil, nil
	}

	// copy secret obj from controller's ns to a new secret
	// that we put in the lab's ns
	newSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      srlLicenseSecretName,
			Namespace: s.Namespace,
		},
		Data: secret.Data,
	}

	log.Info("creating secret",
		"secret name", srlLicenseSecretName,
	)

	err = r.Create(ctx, newSecret)
	if err != nil {
		return nil, err
	}

	return newSecret, err
}

// updateLicenseSecret updates the secret present in Srlinux namespace with the currently
// present secret data.
func (r *SrlinuxReconciler) updateLicenseSecret(
	ctx context.Context,
	_ *srlinuxv1.Srlinux,
	log logr.Logger,
	secret *corev1.Secret,
) (*corev1.Secret, error) {
	mainSecret := &corev1.Secret{} // Secret in controller' namespace we treat as a source of truth

	// get license secret from controller' ns to copy from
	// error if not found.
	err := r.Get(
		ctx,
		types.NamespacedName{Name: srlLicenseSecretName, Namespace: controllerNamespace},
		mainSecret,
	)
	if err != nil && k8serrors.IsNotFound(err) {
		return nil, fmt.Errorf(
			"%w: couldn't find Secret in controller's namespace",
			ErrLicenseProvisioning,
		)
	}

	// if secrets match, don't update the resource
	if cmp.Equal(secret.Data, mainSecret.Data) {
		log.Info("secret already exists, not updating")

		return secret, nil
	}

	log.Info("updating the secret")

	err = r.Update(ctx, secret)
	if err != nil {
		return nil, err
	}

	return secret, err
}
