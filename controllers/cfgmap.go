package controllers

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

// createConfigMaps creates srlinux-variants and srlinux-topomac config maps which every srlinux pod needs to mount.
func createConfigMaps(
	ctx context.Context,
	r *SrlinuxReconciler,
	ns string,
	log logr.Logger,
) error {
	err := createVariantsCfgMap(ctx, r, ns, log)
	if err != nil {
		return err
	}

	err = createTopomacScriptCfgMap(ctx, r, ns, log)
	if err != nil {
		return err
	}

	err = createKNEEntrypointCfgMap(ctx, r, ns, log)

	return err
}

func createVariantsCfgMap(
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

	return err
}

func createTopomacScriptCfgMap(
	ctx context.Context,
	r *SrlinuxReconciler,
	ns string,
	log logr.Logger,
) error {
	// Check if the topomac script cfg map already exists, if not create a new one
	cfgMap := &corev1.ConfigMap{}

	err := r.Get(ctx, types.NamespacedName{Name: topomacCfgMapName, Namespace: ns}, cfgMap)
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

	return err
}

func createKNEEntrypointCfgMap(
	ctx context.Context,
	r *SrlinuxReconciler,
	ns string,
	log logr.Logger,
) error {
	// Check if the kne-entrypoint cfg map already exists, if not create a new one
	cfgMap := &corev1.ConfigMap{}

	err := r.Get(ctx, types.NamespacedName{Name: entrypointCfgMapName, Namespace: ns}, cfgMap)
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

	return err
}
