package controllers

import (
	"context"

	"github.com/go-logr/logr"
	typesv1alpha1 "github.com/srl-labs/srl-controller/api/types/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

// createSecrets creates secrets such as srlinux-licenses.
func (r *SrlinuxReconciler) createSecrets(
	ctx context.Context,
	s *typesv1alpha1.Srlinux,
	log logr.Logger,
) error {
	secret, err := r.copyLicenseSecret(ctx, s, log)
	if err != nil {
		return err
	}

	// set license key matching image version
	s.InitLicenseKey(ctx, secret)

	return err
}

// copyLicenseSecret finds a secret with srlinux licenses in srlinux-controller namespace
// and copies it to the srlinux CR namespace and returns the pointer to the newly created secret.
// If original secret doesn't exist (i.e. when no licenses were provisioned by a user)
// then nothing gets copied and nil returned.
func (r *SrlinuxReconciler) copyLicenseSecret(
	ctx context.Context,
	s *typesv1alpha1.Srlinux,
	log logr.Logger,
) (*corev1.Secret, error) {
	secret := &corev1.Secret{}

	// find license secret in controller' ns to copy from
	// return silently if not found.
	err := r.Get(ctx, types.NamespacedName{Name: srlLicenseSecretName, Namespace: controllerNamespace}, secret)
	if err != nil && errors.IsNotFound(err) {
		log.Info("secret with licenses is not found in controller's namespace, skipping copy to lab namespace",
			"secret name", srlLicenseSecretName,
			"controller namespace", controllerNamespace)

		return nil, nil
	}

	// copy secret obj from controller's ns to a new secret
	// that we put in the lab's ns
	newSecret := secret.DeepCopy()
	newSecret.Namespace = s.Namespace
	newSecret.ResourceVersion = ""

	log.Info("creating secret",
		"secret name", srlLicenseSecretName,
		"namespace", s.Namespace)

	err = r.Create(ctx, newSecret)
	if err != nil {
		return nil, err
	}

	return newSecret, err
}
