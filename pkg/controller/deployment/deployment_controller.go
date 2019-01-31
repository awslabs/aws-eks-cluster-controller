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

package deployment

import (
	"context"
	"reflect"
	"time"

	"github.com/awslabs/aws-eks-cluster-controller/pkg/finalizers"

	appsv1 "k8s.io/api/apps/v1"

	"github.com/awslabs/aws-eks-cluster-controller/pkg/authorizer"

	"github.com/awslabs/aws-eks-cluster-controller/pkg/logging"

	"go.uber.org/zap"

	"github.com/aws/aws-sdk-go/aws/session"

	clusterv1alpha1 "github.com/awslabs/aws-eks-cluster-controller/pkg/apis/cluster/v1alpha1"
	componentsv1alpha1 "github.com/awslabs/aws-eks-cluster-controller/pkg/apis/components/v1alpha1"
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

var (
	DeploymentFinalizer = "deployment.components.eks.amazon.com"
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Deployment Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
// USER ACTION REQUIRED: update cmd/manager/main.go to call this components.Add(mgr) to install this Controller
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	sess := session.Must(session.NewSession())
	log := logging.New()
	return &ReconcileDeployment{
		Client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
		log:    log,
		sess:   sess,
		auth:   authorizer.NewEks(sess, log),
	}
}

// This is for testing.  It will return a reconciler that will use the Client for both local and remote calls.
func newTestReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileDeployment{
		Client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
		log:    logging.New(),
		sess:   session.Must(session.NewSession()),
		auth:   authorizer.NewFake(mgr.GetClient()),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("deployment-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Deployment
	err = c.Watch(&source.Kind{Type: &componentsv1alpha1.Deployment{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileDeployment{}

// ReconcileDeployment reconciles a Deployment object
type ReconcileDeployment struct {
	client.Client
	scheme *runtime.Scheme
	log    *zap.Logger
	sess   *session.Session
	auth   authorizer.Authorizer
}

// Reconcile reads that state of the cluster for a Deployment object and makes changes based on the state read
// and what is in the Deployment.Spec
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=components.eks.amazonaws.com,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.eks.amazonaws.com,resources=eks,verbs=get;list
func (r *ReconcileDeployment) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the Deployment instance
	log := r.log.With(
		zap.String("Name", request.Name),
		zap.String("Namespace", request.Namespace),
		zap.String("Kind", "deployment.components.eks.amazon.com"),
	)
	instance := &componentsv1alpha1.Deployment{}
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

	if len(instance.ObjectMeta.OwnerReferences) < 1 {
		// Add Owner Reference
		instance.ObjectMeta.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion:         "cluster.eks.amazonaws.com/v1alpha1",
				Kind:               "EKS",
				Name:               cluster.ObjectMeta.Name,
				UID:                cluster.ObjectMeta.UID,
				Controller:         func(b bool) *bool { return &b }(true),
				BlockOwnerDeletion: func(b bool) *bool { return &b }(true),
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

	log.Info("got client")
	if !instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if finalizers.HasFinalizer(instance, DeploymentFinalizer) {
			log.Info("deleting deployment")
			found := &appsv1.Deployment{}
			err := client.Get(context.TODO(), remoteKey, found)
			if err != nil && errors.IsNotFound(err) {
				instance.Finalizers = finalizers.RemoveFinalizer(instance, DeploymentFinalizer)
				if err := r.Client.Update(context.TODO(), instance); err != nil {
					return reconcile.Result{}, err
				}
				return reconcile.Result{}, nil
			} else if err != nil {
				log.Error("could not get remote deployment", zap.Error(err))
				return reconcile.Result{}, nil
			}

			if err := client.Delete(context.TODO(), found); err != nil {
				log.Error("could not delete remote deployment", zap.Error(err))
			}
			return reconcile.Result{}, nil

		}
		return reconcile.Result{}, nil
	}

	rDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        instance.Spec.Name,
			Namespace:   instance.Spec.Namespace,
			Labels:      instance.Labels,
			Annotations: instance.Annotations,
		},
		Spec: instance.Spec.GetDeploymentSpec(),
	}
	found := &appsv1.Deployment{}
	err = client.Get(context.TODO(), remoteKey, found)
	if err != nil && errors.IsNotFound(err) {
		log.Info("creating deployment")

		if err := client.Create(context.TODO(), rDeployment); err != nil {
			log.Error("failed to create deployment", zap.Error(err))
			return reconcile.Result{}, err
		}
		instance.Finalizers = []string{DeploymentFinalizer}

		if err := r.Client.Update(context.TODO(), instance); err != nil {
			log.Error("failed to create deployment", zap.Error(err))
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}
	log.Info("found remote deployment")

	if !reflect.DeepEqual(found.Spec, rDeployment.Spec) {
		found.Spec = rDeployment.Spec
		log.Info("updating deployment")
		err := client.Update(context.TODO(), found)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	log.Info("Finished")
	return reconcile.Result{}, nil
}
