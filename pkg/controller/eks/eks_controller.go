package eks

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/awslabs/aws-eks-cluster-controller/pkg/logging"

	"go.uber.org/zap"

	clusterv1alpha1 "github.com/awslabs/aws-eks-cluster-controller/pkg/apis/cluster/v1alpha1"
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

	return nil
}

var _ reconcile.Reconciler = &ReconcileEKS{}

var (
	NodeGroupFinalizer    = "nodegroup.eks.amazonaws.com"
	ControlPlaneFinalizer = "controlplane.eks.amazonaws.com"
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
// +kubebuilder:rbac:groups=cluster.eks.amazonaws.com,resources=controlplane,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.eks.amazonaws.com,resources=nodegroup,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.eks.amazonaws.com,resources=eks,verbs=get;list;watch;create;update;patch;delete
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
			return reconcile.Result{RequeueAfter: 5 * time.Second}, err
		}
		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
	case ControlPlaneUpdating:
		logger.Info("create Control Plane not complete")
		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
	case controlplane.StatusError, controlplane.StatusCreateFailed:
		instance.Status.Status = cpStatus
		err = r.Update(context.TODO(), instance)
		if err != nil {
			return reconcile.Result{RequeueAfter: 5 * time.Second}, err
		}
		return reconcile.Result{}, nil
	case controlplane.StatusCreateComplete:
		// only create nodegroups if controlplane create completes
		break
	default:
		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
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
			return reconcile.Result{RequeueAfter: 5 * time.Second}, err
		}
		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
	}
	if ngStatus == NodeGroupUpdating {
		logger.Info("create NodeGroups not complete")
		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
	}

	instance.Status.Status = "Complete"

	err = r.Update(context.TODO(), instance)
	if err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
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
