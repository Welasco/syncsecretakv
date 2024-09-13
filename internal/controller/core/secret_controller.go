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

	log.Log.Info("Reconciling Secret: " + req.NamespacedName.Name)

	// TODO(user): your logic here
	secret := &corev1.Secret{}
	if err := r.Get(ctx, req.NamespacedName, secret); err != nil && errors.IsNotFound(err) {
		log.Log.Error(err, "Unable to fetch Secret, resource was probably deleted")

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

	if secret.Namespace == "vws" {

		log.Log.Info("Secret Name: " + secret.Name + " Secrete Type: " + string(secret.Type) + "Namespace Name: " + secret.Namespace)

		// check if a secret has a specific label defined
		// if secret.Labels["secret-type"] == "my-secret " {
		// 	log.Log.Info("Secret has a label secret-type: my-secret")
		// }

		// print all labels from a secret
		// for key, value := range secret.Labels {
		// 	log.Log.Info("Label Key: ", key, "\n", "Label Value: ", value)
		// }

		// Get the secret data
		for key, value := range secret.Data {
			log.Log.Info("Key: ", key, "\n", "Value: ", string(value))
		}

		// Print all annotations from a secret
		for key, value := range secret.Annotations {
			log.Log.Info("Annotation Key: ", key, "\n", "Annotation Value: ", value)
		}

		// Print Resource Version of a secret
		//log.Log.Info("Resource Version: ", secret.ResourceVersion)

		// Get the SyncSecretAKV object
		syncSecretAKV := &apiv1alpha1.SyncSecretAKV{}
		//if err := r.Get(ctx, req.NamespacedName, syncSecretAKV); err != nil {
		if err := r.Get(ctx, req.NamespacedName, syncSecretAKV); err != nil {
			log.Log.Info("Error Getting SyncSecretAKV: " + err.Error())
			log.Log.Info("Creating SyncSecretAKV for Secret: " + secret.Name)

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
			//return ctrl.Result{}, client.IgnoreNotFound(err)
		} else {
			// SyncSecretAKV already exist in the cluster, updating it
			// Update if secret.ResourceVersion is different then SyncSecretAKV.Spec.SecretResourceVersion
			if secret.ResourceVersion != syncSecretAKV.Spec.SecretResourceVersion {
				log.Log.Info("Secret Resource Version is different then SyncSecretAKV Secret Resource Version")
				syncSecretAKV.Spec.SecretResourceVersion = secret.ResourceVersion
				if err := r.Update(ctx, syncSecretAKV); err != nil {
					log.Log.Error(err, "Unable to Update SyncSecretAKV")
					//return ctrl.Result{}, err
				}
				log.Log.Info("Successfully Updated SyncSecretAKV with new Secret Resource Version: " + syncSecretAKV.Spec.SecretResourceVersion)
			} else {
				log.Log.Info("Secret Resource Version is the same as SyncSecretAKV Secret Resource Version")
			}
		}

		// if (&apiv1alpha1.SyncSecretAKV{}) != syncSecretAKV {
		// 	log.Log.Info("\nSyncSecretAKV Vault Name: " + syncSecretAKV.Spec.VaultName + "\nSyncSecretAKV Secret Name: " + syncSecretAKV.Spec.SecretName + " \nSyncSecretAKV Secret Resource Version: " + syncSecretAKV.Spec.SecretResourceVersion)
		// }

		// Update if secret.ResourceVersion is different then SyncSecretAKV.Spec.SecretResourceVersion
		// if secret.ResourceVersion != syncSecretAKV.Spec.SecretResourceVersion {
		// 	log.Log.Info("Secret Resource Version is different then SyncSecretAKV Secret Resource Version")
		// 	syncSecretAKV.Spec.SecretResourceVersion = secret.ResourceVersion
		// 	if err := r.Update(ctx, syncSecretAKV); err != nil {
		// 		log.Log.Error(err, "Unable to Update SyncSecretAKV")
		// 		//return ctrl.Result{}, err
		// 	}
		// 	log.Log.Info("Successfully Updated SyncSecretAKV with new Secret Resource Version: " + syncSecretAKV.Spec.SecretResourceVersion)
		// } else {
		// 	log.Log.Info("Secret Resource Version is the same as SyncSecretAKV Secret Resource Version")
		// }

	}

	// syncSecretAKV := &apiv1alpha1.SyncSecretAKV{}
	// if err := r.Get(ctx, req.NamespacedName, syncSecretAKV); err != nil {
	// 	fmt.Printf("Error Getting SyncSecretAKV: %s\n", err)
	// 	//return ctrl.Result{}, client.IgnoreNotFound(err)
	// }
	// if (&apiv1alpha1.SyncSecretAKV{}) != syncSecretAKV {
	// 	fmt.Printf("SyncSecretAKV Name: %s \nSyncSecretAKV Vault Name: %s \nSyncSecretAKV Secret Name: %s \nSyncSecretAKV Secret Resource Version: %s\n", syncSecretAKV.Spec.Name, syncSecretAKV.Spec.VaultName, syncSecretAKV.Spec.SecretName, syncSecretAKV.Spec.SecretResourceVersion)
	// }

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		Complete(r)
}
