package eks

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/types"

	"github.com/awslabs/aws-eks-cluster-controller/pkg/logging"

	"go.uber.org/zap"

	clusterv1alpha1 "github.com/awslabs/aws-eks-cluster-controller/pkg/apis/cluster/v1alpha1"
	componentsv1alpha1 "github.com/awslabs/aws-eks-cluster-controller/pkg/apis/components/v1alpha1"
	"github.com/awslabs/aws-eks-cluster-controller/pkg/controller/controlplane"
	"github.com/awslabs/aws-eks-cluster-controller/pkg/finalizers"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new EKS Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
// USER ACTION REQUIRED: update cmd/manager/main.go to call this cluster.Add(mgr) to install this Controller
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileEKS{Client: mgr.GetClient(), scheme: mgr.GetScheme(), log: logging.New()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("eks-controller", mgr, controller.Options{Reconciler: r, MaxConcurrentReconciles: 10})
	if err != nil {
		return err
	}

	// Watch for changes to EKS
	err = c.Watch(&source.Kind{Type: &clusterv1alpha1.EKS{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{
		Type: &clusterv1alpha1.ControlPlane{}},
		&handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &clusterv1alpha1.EKS{},
		})
	if err != nil {
		return err
	}
	err = c.Watch(&source.Kind{
		Type: &clusterv1alpha1.NodeGroup{}},
		&handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &clusterv1alpha1.EKS{},
		})
	if err != nil {
		return err
	}
	err = c.Watch(&source.Kind{
		Type: &componentsv1alpha1.ConfigMap{}},
		&handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &clusterv1alpha1.EKS{},
		})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileEKS{}

var (
	NodeGroupFinalizer    = "nodegroup.eks.amazonaws.com"
	ControlPlaneFinalizer = "controlplane.eks.amazonaws.com"
	ComponentsFinalizer   = "components.eks.amazonaws.com"
)

// ReconcileEKS reconciles a EKS object
type ReconcileEKS struct {
	client.Client
	scheme *runtime.Scheme
	log    *zap.Logger
}

// Reconcile reads that state of the cluster for a EKS object and makes changes based on the state read
// and what is in the EKS.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  The scaffolding writes
// a Deployment as an example
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=cluster.eks.amazonaws.com,resources=controlplanes;nodegroups;ekss,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=components.eks.amazonaws.com,resources=configmaps,verbs=get;list;watch;create;update;patch;delete

func (r *ReconcileEKS) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the EKS instance
	logger := r.log.With(
		zap.String("Kind", "EKS"),
		zap.String("Name", request.Name),
		zap.String("NameSpace", request.Namespace),
	)
	defer logger.Sync()

	instance := &clusterv1alpha1.EKS{}
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

	logger.Info("got reconcile request")

	if !instance.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info("deleting eks")

		if finalizers.HasFinalizer(instance, ComponentsFinalizer) {
			result, err := r.deleteComponents(instance, logger)
			if err != nil {
				logger.Error("Error Deleting Components", zap.Error(err))
			}
			return result, err
		}

		if err := r.deleteConfigmap(instance); err != nil {
			logger.Error("Error Deleting Configmap", zap.Error(err))
			return reconcile.Result{}, err
		}

		if finalizers.HasFinalizer(instance, NodeGroupFinalizer) {
			result, err := r.deleteNodeGroups(instance, logger)
			if err != nil {
				logger.Error("Error Deleting Node Groups", zap.Error(err))
			}
			return result, err
		}

		if finalizers.HasFinalizer(instance, ControlPlaneFinalizer) {
			result, err := r.deleteControlPlane(instance, logger)
			if err != nil {
				logger.Error("Error Deleting Control Plane", zap.Error(err))
			}
			return result, err
		}

		return reconcile.Result{}, nil
	}

	cpStatus, err := r.createControlPlane(instance)
	if err != nil {
		logger.Error("create Control Plane Failed", zap.Error(err))
		return reconcile.Result{}, err
	}

	switch cpStatus {
	case ControlPlaneCreating:
		instance.SetFinalizers(finalizers.AddFinalizer(instance, ControlPlaneFinalizer))
		instance.Status.Status = "Creating Control Plane"
		err = r.Update(context.TODO(), instance)
		if err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	case ControlPlaneUpdating:
		logger.Info("create Control Plane not complete")
		return reconcile.Result{}, nil
	case controlplane.StatusError, controlplane.StatusFailed:
		instance.Status.Status = cpStatus
		err = r.Update(context.TODO(), instance)
		if err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	case controlplane.StatusCreateComplete:
		// only create nodegroups if controlplane create completes
		break
	default:
		return reconcile.Result{}, nil
	}

	ngStatus, err := r.createAllNodeGroups(instance)
	if err != nil {
		logger.Error("error creating NodeGroups", zap.Error(err))
		return reconcile.Result{}, err
	}

	if ngStatus == NodeGroupCreating {
		instance.SetFinalizers(finalizers.AddFinalizer(instance, NodeGroupFinalizer))
		instance.Status.Status = "Creating NodeGroups"
		err = r.Update(context.TODO(), instance)
		if err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}
	if ngStatus == NodeGroupUpdating {
		logger.Info("create NodeGroups not complete")
		return reconcile.Result{}, nil
	}

	err = r.createConfigMap(instance)
	if err != nil {
		logger.Error("failed to create configmap")
		return reconcile.Result{}, err
	}

	if instance.Status.Status != "Complete" {
		instance.Status.Status = "Complete"

		err = r.Update(context.TODO(), instance)
		if err != nil {
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}

func (r *ReconcileEKS) createConfigMap(instance *clusterv1alpha1.EKS) error {
	labels := map[string]string{
		"eks.owner":           fmt.Sprintf("%s_%s", instance.Namespace, instance.Name),
		"eks.owner.name":      instance.Name,
		"eks.owner.namespace": instance.Namespace,
	}
	for k, v := range instance.Labels {
		labels[k] = v
	}

	configMap := &componentsv1alpha1.ConfigMap{}
	err := r.Get(context.TODO(), types.NamespacedName{Namespace: instance.Namespace, Name: instance.Name + "-configmap-aws-auth"}, configMap)
	if err != nil && errors.IsNotFound(err) {
		configMap = &componentsv1alpha1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:        instance.Name + "-configmap-aws-auth",
				Namespace:   instance.Namespace,
				Labels:      labels,
				Annotations: instance.Annotations,
			},
			Spec: componentsv1alpha1.ConfigMapSpec{
				Name:      "aws-auth",
				Namespace: "kube-system",
				Cluster:   instance.Name,
				Data: map[string]string{
					"mapRoles": instance.GetAWSAuthData(),
				},
			},
		}
		r.log.Info("Creating configmap", zap.String("Configmap", fmt.Sprintf("%+v", *configMap)))
		err = r.Create(context.TODO(), configMap)
		if err != nil {
			return err
		}

	}
	if err != nil {
		return err
	}
	return nil
}

func (r *ReconcileEKS) deleteConfigmap(instance *clusterv1alpha1.EKS) error {
	configmap := &componentsv1alpha1.ConfigMap{}
	configmapKey := types.NamespacedName{Namespace: instance.Namespace, Name: instance.Name + "-configmap-aws-auth"}
	err := r.Get(context.TODO(), configmapKey, configmap)
	if err != nil && errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	err = r.Delete(context.TODO(), configmap)
	if err != nil {
		return err
	}
	return nil

}

func (r *ReconcileEKS) createControlPlane(instance *clusterv1alpha1.EKS) (string, error) {
	labels := map[string]string{
		"eks.owner":           fmt.Sprintf("%s_%s", instance.Namespace, instance.Name),
		"eks.owner.name":      instance.Name,
		"eks.owner.namespace": instance.Namespace,
	}
	for k, v := range instance.Labels {
		labels[k] = v
	}

	cp := &clusterv1alpha1.ControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:        instance.Name + "-controlplane",
			Namespace:   instance.Namespace,
			Labels:      labels,
			Annotations: instance.Annotations,
		},
		Spec: clusterv1alpha1.ControlPlaneSpec{
			ClusterName: instance.Spec.ControlPlane.ClusterName,
		},
	}
	if err := controllerutil.SetControllerReference(instance, cp, r.scheme); err != nil {
		return "", err
	}
	logger := r.log.With(
		zap.String("Kind", "EKS"),
		zap.String("Name", instance.Name),
		zap.String("NameSpace", instance.Namespace),
		zap.String("ControlPlane", cp.GetName()),
	)

	status, err := createOrUpdateControlPlane(cp, r)
	if err != nil {
		return "", err
	}

	logger.Info("reconciling ControlPlane", zap.String("Status", status))
	return status, nil
}

func (r *ReconcileEKS) createAllNodeGroups(instance *clusterv1alpha1.EKS) (string, error) {
	logger := r.log.With(
		zap.String("Kind", "EKS"),
		zap.String("Name", instance.Name),
		zap.String("NameSpace", instance.Namespace),
	)
	labels := map[string]string{
		"eks.owner":           fmt.Sprintf("%s_%s", instance.Namespace, instance.Name),
		"eks.owner.name":      instance.Name,
		"eks.owner.namespace": instance.Namespace,
	}
	for k, v := range instance.Labels {
		labels[k] = v
	}

	nodeGroups := make([]*clusterv1alpha1.NodeGroup, 0, len(instance.Spec.NodeGroups))

	for _, nodeGroupSpec := range instance.Spec.NodeGroups {
		nodeGroup := &clusterv1alpha1.NodeGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:        strings.ToLower(fmt.Sprintf("%s-nodegroup-%s", instance.Name, nodeGroupSpec.Name)),
				Namespace:   instance.Namespace,
				Labels:      labels,
				Annotations: instance.Annotations,
			},
			Spec: nodeGroupSpec,
		}

		if err := controllerutil.SetControllerReference(instance, nodeGroup, r.scheme); err != nil {
			return "", err
		}
		nodeGroups = append(nodeGroups, nodeGroup)
	}
	status, err := createOrUpdateNodegroups(nodeGroups, r)
	if err != nil {
		return "", err
	}
	if status == NodeGroupCreating {
		logger.Info("Creating NodeGroups")
	}
	logger.Info("reconciling Node Groups", zap.String("Status", status))

	return status, nil
}

func (r *ReconcileEKS) deleteNodeGroups(instance *clusterv1alpha1.EKS, logger *zap.Logger) (reconcile.Result, error) {

	nodeGroups := &clusterv1alpha1.NodeGroupList{}

	err := r.List(context.TODO(),
		client.MatchingLabels(map[string]string{
			"eks.owner.name":      instance.Name,
			"eks.owner.namespace": instance.Namespace,
		}),
		nodeGroups)
	if err != nil {
		return reconcile.Result{}, err
	}

	if len(nodeGroups.Items) == 0 {
		logger.Info("Node Groups removed")
		instance.Finalizers = finalizers.RemoveFinalizer(instance, NodeGroupFinalizer)
		err = r.Update(context.TODO(), instance)
		if err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}

	for _, nodeGroup := range nodeGroups.Items {
		logger.Info("deleting Node Group", zap.String("NodeGroup", nodeGroup.Name))
		err = r.Delete(context.TODO(), &nodeGroup)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{Requeue: true}, nil
}

func (r *ReconcileEKS) deleteControlPlane(instance *clusterv1alpha1.EKS, logger *zap.Logger) (reconcile.Result, error) {
	controlPlanes := &clusterv1alpha1.ControlPlaneList{}

	err := r.List(context.TODO(),
		client.MatchingLabels(map[string]string{
			"eks.owner.name":      instance.Name,
			"eks.owner.namespace": instance.Namespace,
		}),
		controlPlanes)
	if err != nil {
		return reconcile.Result{}, err
	}

	if len(controlPlanes.Items) == 0 {
		logger.Info("Control Plane removed")
		instance.Finalizers = finalizers.RemoveFinalizer(instance, ControlPlaneFinalizer)
		err = r.Update(context.TODO(), instance)
		if err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}

	for _, controlPlane := range controlPlanes.Items {
		logger.Info("deleting Control Plane", zap.String("ControlPlane", controlPlane.Name))

		err = r.Delete(context.TODO(), &controlPlane)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	instance.Status.Status = "Deleting Control Plane"
	err = r.Update(context.TODO(), instance)
	if err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{Requeue: true}, nil
}

func (r *ReconcileEKS) deleteComponents(instance *clusterv1alpha1.EKS, logger *zap.Logger) (reconcile.Result, error) {

	count, err := deleteComponents(instance.Name, instance.Namespace, r, logger)
	if err != nil {
		logger.Error("error deleting components", zap.Error(err))
		return reconcile.Result{}, err
	}
	if count == 0 {
		instance.Finalizers = finalizers.RemoveFinalizer(instance, ComponentsFinalizer)
		err := r.Update(context.TODO(), instance)
		if err != nil {
			logger.Error("error removing Finalizer", zap.Error(err))
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}
	return reconcile.Result{Requeue: true}, nil
}
