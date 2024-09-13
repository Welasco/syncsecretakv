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

package api

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apiv1alpha1 "github.com/welasco/syncsecretakv/api/api/v1alpha1"
)

// SyncSecretAKVReconciler reconciles a SyncSecretAKV object
type SyncSecretAKVReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=api.syncsecretakv.io,resources=syncsecretakvs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=api.syncsecretakv.io,resources=syncsecretakvs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=api.syncsecretakv.io,resources=syncsecretakvs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the SyncSecretAKV object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *SyncSecretAKVReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	log.Log.Info("Reconciling SyncSecretAKV: " + req.NamespacedName.Name)

	// TODO(user): your logic here
	syncSecretAKV := &apiv1alpha1.SyncSecretAKV{}
	if err := r.Get(ctx, req.NamespacedName, syncSecretAKV); err != nil && errors.IsNotFound(err) {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Log.Info("SyncSecretAKV detected (add, update, delete): " + syncSecretAKV.Name)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SyncSecretAKVReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1alpha1.SyncSecretAKV{}).
		Complete(r)
}
