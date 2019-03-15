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

package daemonset

import (
	"context"
	"reflect"

	"github.com/aws/aws-sdk-go/aws/session"
	clusterv1alpha1 "github.com/awslabs/aws-eks-cluster-controller/pkg/apis/cluster/v1alpha1"
	componentsv1alpha1 "github.com/awslabs/aws-eks-cluster-controller/pkg/apis/components/v1alpha1"
	"github.com/awslabs/aws-eks-cluster-controller/pkg/authorizer"
	"github.com/awslabs/aws-eks-cluster-controller/pkg/finalizers"
	"github.com/awslabs/aws-eks-cluster-controller/pkg/logging"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"time"
)

var (
	DaemonSetFinalizer = "daemonset.components.eks.amazon.com"
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new DaemonSet Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
// USER ACTION REQUIRED: update cmd/manager/main.go to call this components.Add(mgr) to install this Controller
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	sess := session.Must(session.NewSession())
	log := logging.New()
	return &ReconcileDaemonSet{
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
	c, err := controller.New("daemonset-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to DaemonSet
	err = c.Watch(&source.Kind{Type: &componentsv1alpha1.DaemonSet{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileDaemonSet{}

// ReconcileDaemonSet reconciles a DaemonSet object
type ReconcileDaemonSet struct {
	client.Client
	scheme *runtime.Scheme
	log    *zap.Logger
	sess   *session.Session
	auth   authorizer.Authorizer
}

// Reconcile reads that state of the cluster for a DaemonSet object and makes changes based on the state read
// and what is in the DaemonSet.Spec

// +kubebuilder:rbac:groups=components.eks.amazonaws.com,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcileDaemonSet) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log := r.log.With(
		zap.String("Name", request.Name),
		zap.String("Namespace", request.Namespace),
		zap.String("Kind", "daemonset.components.eks.amazon.com"),
	)
	// Fetch the DaemonSet instance
	instance := &componentsv1alpha1.DaemonSet{}
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{Requeue: false}, err
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

	if err := controllerutil.SetControllerReference(cluster, instance, r.scheme); err != nil {
		return reconcile.Result{}, nil
	}

	client, err := r.auth.GetClient(cluster)
	if err != nil {
		log.Error("could not access remote cluster", zap.Error(err))
		return reconcile.Result{}, err
	}

	log.Info("got client")

	if !instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if finalizers.HasFinalizer(instance, DaemonSetFinalizer) {
			log.Info("deleting daemonset")
			found := &appsv1.DaemonSet{}
			err := client.Get(context.TODO(), remoteKey, found)
			if err != nil && errors.IsNotFound(err) {
				instance.Finalizers = finalizers.RemoveFinalizer(instance, DaemonSetFinalizer)
				if err := r.Client.Update(context.TODO(), instance); err != nil {
					return reconcile.Result{}, err
				}
				return reconcile.Result{}, nil
			} else if err != nil {
				log.Error("could not get remote daemonset", zap.Error(err))
				return reconcile.Result{}, nil
			}

			if err := client.Delete(context.TODO(), found); err != nil {
				log.Error("could not delete remote daemonset", zap.Error(err))
			}
			return reconcile.Result{RequeueAfter: 5 * time.Second}, nil

		}
		return reconcile.Result{}, nil
	}

	rDaemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        instance.Spec.Name,
			Namespace:   instance.Spec.Namespace,
			Labels:      instance.Labels,
			Annotations: instance.Annotations,
		},
		Spec: instance.Spec.DaemonSetSpec,
	}

	found := &appsv1.DaemonSet{}
	err = client.Get(context.TODO(), remoteKey, found)
	if err != nil && errors.IsNotFound(err) {
		log.Info("creating daemonset")

		if err := client.Create(context.TODO(), rDaemonSet); err != nil {
			log.Error("failed to create remote daemonset", zap.Error(err))
			return reconcile.Result{}, err
		}
		instance.Finalizers = []string{DaemonSetFinalizer}
		instance.Status.Status = "Created"

		if err := r.Client.Update(context.TODO(), instance); err != nil {
			log.Error("failed to update daemonset", zap.Error(err))
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}
	log.Info("found remote daemonset")

	if !reflect.DeepEqual(found.Spec, rDaemonSet.Spec) {
		found.Spec = rDaemonSet.Spec
		log.Info("updating daemonset")
		err := client.Update(context.TODO(), found)
		if err != nil {
			log.Error("failed to update remote daemonset", zap.Error(err))
			return reconcile.Result{}, err
		}
		log.Info("daemonset updated")
	}

	instance.Status.DaemonSetStatus = found.Status
	if err := r.Client.Update(context.TODO(), instance); err != nil {
		log.Error("failed to update status of daemonset", zap.Error(err))
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}
