/*
Copyright 2024 welasco.

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

package core

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apiv1alpha1 "github.com/welasco/syncsecretakv/api/api/v1alpha1"
	"github.com/welasco/syncsecretakv/internal/controller/api"
)

// SecretReconciler reconciles a Secret object
type SecretReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=secrets/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Secret object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *SecretReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// TODO(user): your logic here

	if req.NamespacedName.Namespace == "vws" {
		log.Log.Info("Reconciling Secret: " + req.NamespacedName.Name + ", Namespace: " + req.NamespacedName.Namespace)

		// Load the Config object from the namespace
		// LoadConfig function is defined in the api package at internal/controller/api/config_controller.go
		config, err := api.LoadConfig(ctx, r.Client)
		if err != nil {
			log.Log.Error(err, "Config not found. Unable to load Config resource from namespace: "+req.NamespacedName.Namespace)
			return ctrl.Result{}, err
		}

		secret := &corev1.Secret{}
		if err := r.Get(ctx, req.NamespacedName, secret); err != nil && errors.IsNotFound(err) {
			log.Log.Info("Unable to fetch Secret, resource was probably deleted. Secret: " + req.NamespacedName.Name + ", Namespace: " + req.NamespacedName.Namespace)

			// Check if there is a SyncSecretAKV associated with the secret
			// If there is, delete the SyncSecretAKV
			deleteSyncSecretAKV := &apiv1alpha1.SyncSecretAKV{}
			if err := r.Get(ctx, req.NamespacedName, deleteSyncSecretAKV); err != nil && errors.IsNotFound(err) {
				log.Log.Error(err, "Unable to fetch SyncSecretAKV, resource was probably deleted")
			} else {
				if err := r.Delete(ctx, deleteSyncSecretAKV); err != nil {
					log.Log.Error(err, "Unable to delete SyncSecretAKV")
				}
				log.Log.Info("Successfully Deleted SyncSecretAKV: " + deleteSyncSecretAKV.Name)
			}

			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		// Check if Secret type is kubernetes.io/tls
		if secret.Type != "kubernetes.io/tls" {
			log.Log.Info("Secret Type is not kubernetes.io/tls, Ignoring Secret. Secret Name: " + secret.Name + " Secrete Type: " + string(secret.Type) + "Namespace Name: " + secret.Namespace)
			return ctrl.Result{}, nil
		}

		// Check if the secret has all the labels from a Config.FilterMathincgLabels object
		for key, value := range config.Spec.FilterMatchingLabels {
			if secret.Labels[key] != value {
				log.Log.Info("Label not found in Secret: " + secret.Name + ", Label Key: " + key + " Label Value " + value + ". Ignoring the Secret because of label mismatch comparing with Config FilterMatchingLabels")
				return ctrl.Result{}, nil
			}
		}

		// Check if the secret has all the annotations from a Config.FilterMatchingAnnotations object
		for key, value := range config.Spec.FilterMatchingAnnotations {
			if secret.Annotations[key] != value {
				log.Log.Info("Annotation not found in Secret: " + secret.Name + ", Annotation Key: " + key + " Annotation Value " + value + ". Ignoring the Secret because of Annotation mismatch comparing with Config FilterMatchingAnnotations")
				return ctrl.Result{}, nil
			}
		}

		/////////////////////////////////////////////////////////////////////////////////////

		// Get the SyncSecretAKV object
		syncSecretAKV := &apiv1alpha1.SyncSecretAKV{}
		if err := r.Get(ctx, req.NamespacedName, syncSecretAKV); err != nil {
			log.Log.Info("New Secret Detected! SyncSecretAKV not found, Creating SyncSecretAKV for Secret: " + secret.Name)

			// Create a new SyncSecretAKV object
			newSyncSecretAKV := &apiv1alpha1.SyncSecretAKV{
				ObjectMeta: ctrl.ObjectMeta{
					Name:      secret.Name,
					Namespace: secret.Namespace,
				},
				Spec: apiv1alpha1.SyncSecretAKVSpec{
					VaultName:             "my-vault",
					SecretName:            secret.Name,
					SecretResourceVersion: secret.ResourceVersion,
				},
			}
			// Create the SyncSecretAKV resource in the cluster
			if err := r.Create(ctx, newSyncSecretAKV); err != nil {
				log.Log.Error(err, "Unable to create SyncSecretAKV")
				//return ctrl.Result{}, err
			}
			log.Log.Info("Successfully Created SyncSecretAKV: " + newSyncSecretAKV.Name)
			//return ctrl.Result{}, client.IgnoreNotFound(err)
		} else {
			// SyncSecretAKV already exist in the cluster, updating it
			// Update if secret.ResourceVersion is different then SyncSecretAKV.Spec.SecretResourceVersion
			if secret.ResourceVersion != syncSecretAKV.Spec.SecretResourceVersion {
				log.Log.Info("Secret Update detected, Updating SyncSecretAKV with new Secret Resource Version")
				syncSecretAKV.Spec.SecretResourceVersion = secret.ResourceVersion
				if err := r.Update(ctx, syncSecretAKV); err != nil {
					log.Log.Error(err, "Unable to Update SyncSecretAKV")
					//return ctrl.Result{}, err
				}
				log.Log.Info("Successfully Updated SyncSecretAKV with new Secret Resource Version: " + syncSecretAKV.Spec.SecretResourceVersion)
			} else {
				log.Log.Info("Secret not changed, no need to update SyncSecretAKV")
			}
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		Complete(r)
}
