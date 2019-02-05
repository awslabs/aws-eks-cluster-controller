// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package serviceaccount

import (
	"context"
	"reflect"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	clusterv1alpha1 "github.com/awslabs/aws-eks-cluster-controller/pkg/apis/cluster/v1alpha1"
	componentsv1alpha1 "github.com/awslabs/aws-eks-cluster-controller/pkg/apis/components/v1alpha1"
	"github.com/awslabs/aws-eks-cluster-controller/pkg/authorizer"
	"github.com/awslabs/aws-eks-cluster-controller/pkg/finalizers"
	"github.com/awslabs/aws-eks-cluster-controller/pkg/logging"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	ServiceAccountFinalizer = "serviceaccount.components.eks.amazon.com"
)

// Add creates a new ServiceAccount Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	sess := session.Must(session.NewSession())
	log := logging.New()
	return &ReconcileServiceAccount{
		Client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
		log:    log,
		sess:   sess,
		auth:   authorizer.NewEks(sess, log),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("serviceaccount-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to ServiceAccount
	err = c.Watch(&source.Kind{Type: &componentsv1alpha1.ServiceAccount{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileServiceAccount{}

// ReconcileServiceAccount reconciles a ServiceAccount object
type ReconcileServiceAccount struct {
	client.Client
	scheme *runtime.Scheme
	log    *zap.Logger
	sess   *session.Session
	auth   authorizer.Authorizer
}

// Reconcile reads that state of the cluster for a ServiceAccount object and makes changes based on the state read
// and what is in the ServiceAccount.Spec
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=components.eks.amazonaws.com,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=components.eks.amazonaws.com,resources=serviceaccounts/status,verbs=get;update;patch
func (r *ReconcileServiceAccount) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log := r.log.With(
		zap.String("Name", request.Name),
		zap.String("Namespace", request.Namespace),
		zap.String("Kind", "serviceaccount.components.eks.amazon.com"),
	)

	// Fetch the ServiceAccount instance
	instance := &componentsv1alpha1.ServiceAccount{}
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	remoteKey := types.NamespacedName{Namespace: instance.Spec.Namespace, Name: instance.Spec.Name}

	cluster := &clusterv1alpha1.EKS{}
	clusterKey := types.NamespacedName{Name: instance.Spec.Cluster, Namespace: instance.Namespace}
	if err := r.Get(context.TODO(), clusterKey, cluster); err != nil {
		instance.Finalizers = []string{}
		instance.Status.Status = "EKS Cluster not found"
		log.Error("EKS cluster not found", zap.Error(err))
		r.Update(context.TODO(), instance)
		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
	}
	log.Info("got cluster", zap.String("ClusterName", cluster.Name))

	if len(instance.ObjectMeta.OwnerReferences) < 1 {
		// Add Owner Reference
		instance.ObjectMeta.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion: "cluster.eks.amazonaws.com/v1alpha1",
				Kind:       "EKS",
				Name:       cluster.ObjectMeta.Name,
				UID:        cluster.ObjectMeta.UID,
			},
		}
		if err := r.Client.Update(context.TODO(), instance); err != nil {
			return reconcile.Result{}, err
		}
	}

	client, err := r.auth.GetClient(cluster)
	if err != nil {
		log.Error("could not access remote cluster", zap.Error(err))
		return reconcile.Result{}, err
	}

	if !instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if finalizers.HasFinalizer(instance, ServiceAccountFinalizer) {
			log.Info("deleting serviceaccount")
			found := &corev1.ServiceAccount{}
			err := client.Get(context.TODO(), remoteKey, found)
			if err != nil && errors.IsNotFound(err) {
				instance.Finalizers = finalizers.RemoveFinalizer(instance, ServiceAccountFinalizer)
				if err := r.Client.Update(context.TODO(), instance); err != nil {
					return reconcile.Result{}, err
				}
				return reconcile.Result{}, nil
			} else if err != nil {
				log.Error("could not get remote serviceaccount", zap.Error(err))
				return reconcile.Result{}, nil
			}

			if err := client.Delete(context.TODO(), found); err != nil {
				log.Error("could not delete remote serviceaccount", zap.Error(err))
			}
			return reconcile.Result{}, nil

		}
		return reconcile.Result{}, nil
	}

	rServiceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        instance.Spec.Name,
			Namespace:   instance.Spec.Namespace,
			Labels:      instance.Spec.Labels,
			Annotations: instance.Spec.Annotations,
		},
		Secrets:                      instance.Spec.Secrets,
		ImagePullSecrets:             instance.Spec.ImagePullSecrets,
		AutomountServiceAccountToken: instance.Spec.AutomountServiceAccountToken,
	}

	found := &corev1.ServiceAccount{}
	err = client.Get(context.TODO(), remoteKey, found)
	if err != nil && errors.IsNotFound(err) {
		log.Info("creating serviceaccount")

		if err := client.Create(context.TODO(), rServiceAccount); err != nil {
			log.Error("failed to create remote serviceaccount", zap.Error(err))
			return reconcile.Result{}, err
		}
		instance.Finalizers = []string{ServiceAccountFinalizer}
		instance.Status.Status = "Created"
		if err := r.Client.Update(context.TODO(), instance); err != nil {
			log.Error("failed to create serviceaccount", zap.Error(err))
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	if !reflect.DeepEqual(found.Secrets, instance.Spec.Secrets) ||
		!reflect.DeepEqual(found.ImagePullSecrets, instance.Spec.ImagePullSecrets) ||
		!reflect.DeepEqual(found.AutomountServiceAccountToken, instance.Spec.AutomountServiceAccountToken) {
		found.Secrets = instance.Spec.Secrets
		found.ImagePullSecrets = instance.Spec.ImagePullSecrets
		found.AutomountServiceAccountToken = instance.Spec.AutomountServiceAccountToken
		log.Info("updating serviceaccount")
		err := client.Update(context.TODO(), found)
		if err != nil {
			log.Error("failed to update remote serviceaccount", zap.Error(err))
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}
