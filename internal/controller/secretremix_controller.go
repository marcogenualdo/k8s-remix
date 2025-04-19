/*
Copyright 2025.

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

package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	remixv1alpha1 "github.com/marcogenualdo/k8s-remix/api/v1alpha1"
)

// SecretRemixReconciler reconciles a SecretRemix object
type SecretRemixReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=remix.openkube.io,resources=secretremixes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=remix.openkube.io,resources=secretremixes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=remix.openkube.io,resources=secretremixes/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.2/pkg/reconcile
func (r *SecretRemixReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling SecretRemix", "name", req.Name, "namespace", req.Namespace)

	// Fetch the SecretRemix instance
	secretRemix := &remixv1alpha1.SecretRemix{}
	if err := r.Get(ctx, req.NamespacedName, secretRemix); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("SecretRemix resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get SecretRemix")
		return ctrl.Result{}, err
	}

	// set secretRemix status
	secretRemix.Status.Conditions = []metav1.Condition{
		{
			Type:    "Syncing",
			Status:  metav1.ConditionTrue,
			Reason:  "Syncing",
			Message: "Syncing",
		},
	}
	if err := r.Status().Update(ctx, secretRemix); err != nil {
		logger.Error(err, "Failed to update SecretRemix status")
		return ctrl.Result{}, err
	}

	type SecretData map[string][]byte
	var secretData = SecretData{}
	var watchedResources = []remixv1alpha1.WatchedResource{}

	// iterate over dataFrom
	for _, item := range secretRemix.DataFrom {
		if item.Value != "" {
			secretData[item.Key] = []byte(item.Value)
		} else if item.ValueFrom != nil && item.ValueFrom.ConfigMapKeyRef != nil {
			ref := item.ValueFrom.ConfigMapKeyRef
			if ref.Namespace == "" {
				ref.Namespace = secretRemix.Namespace
			}
			configMap := &corev1.ConfigMap{}
			configMapRef := types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}
			err := r.Get(ctx, configMapRef, configMap)
			if err != nil {
				logger.Error(err, "Failed to get ConfigMap")
				return ctrl.Result{}, err
			}

			secretData[item.Key] = []byte(configMap.Data[ref.Key])
			watchedResources = append(watchedResources, remixv1alpha1.WatchedResource{Name: ref.Name, Namespace: ref.Namespace})
		} else if item.ValueFrom != nil && item.ValueFrom.SecretKeyRef != nil {
			ref := item.ValueFrom.SecretKeyRef
			if ref.Namespace == "" {
				ref.Namespace = secretRemix.Namespace
			}
			secret := &corev1.Secret{}
			secretRef := types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}
			err := r.Get(ctx, secretRef, secret)
			if err != nil {
				logger.Error(err, "Failed to get Secret")
				return ctrl.Result{}, err
			}

			secretData[item.Key] = secret.Data[ref.Key]
			watchedResources = append(watchedResources, remixv1alpha1.WatchedResource{Name: ref.Name, Namespace: ref.Namespace})
		} else {
			err := fmt.Errorf("Value not found and ValueFrom must be either a ConfigMapKeyRef or a SecretKeyRef")
			logger.Error(err, "SecretRemix error.")
			return ctrl.Result{}, err
		}
	}

	// create or update secret
	remixedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretRemix.Name,
			Namespace: secretRemix.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
	}
	result, err := ctrl.CreateOrUpdate(ctx, r.Client, remixedSecret, func() error {
		remixedSecret.Data = secretData
		return nil
	})
	if err != nil {
		logger.Error(err, "Failed to create Secret")
		return ctrl.Result{}, err
	}

	// update SecretRemix status
	secretRemix.Status.WatchedResources = watchedResources
	secretRemix.Status.Conditions = []metav1.Condition{
		{
			Type:    "Ready",
			Status:  metav1.ConditionTrue,
			Reason:  "SecretRemixReady",
			Message: "SecretRemix is ready",
		},
	}
	if err := r.Status().Update(ctx, secretRemix); err != nil {
		logger.Error(err, "Failed to update SecretRemix status")
		return ctrl.Result{}, err
	}

	logger.Info("Secret reconciled", "result", result)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SecretRemixReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&remixv1alpha1.SecretRemix{}).
		Watches(
			&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(r.findSecretRemixes),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.findSecretRemixes),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Named("secretremix").
		Complete(r)
}

func (r *SecretRemixReconciler) findSecretRemixes(ctx context.Context, secret client.Object) []reconcile.Request {
	return make([]reconcile.Request)
}
