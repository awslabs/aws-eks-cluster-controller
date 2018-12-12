package nodegroup

import (
	"context"
	"fmt"

	clusterv1alpha1 "github.com/awslabs/aws-eks-cluster-controller/pkg/apis/cluster/v1alpha1"
	"github.com/awslabs/aws-eks-cluster-controller/pkg/cfnhelper"
	"github.com/awslabs/aws-eks-cluster-controller/pkg/finalizers"
	"github.com/awslabs/aws-eks-cluster-controller/pkg/logging"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileNodeGroup{
		Client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
		log:    logging.New(),
		sess:   session.Must(session.NewSession()),
		cfnSvc: nil,
	}
}

// ReconcileNodeGroup reconciles a NodeGroup object
type ReconcileNodeGroup struct {
	client.Client
	scheme *runtime.Scheme
	log    *zap.Logger
	sess   *session.Session
	cfnSvc cloudformationiface.CloudFormationAPI
}

// Add creates a new NodeGroup Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
// USER ACTION REQUIRED: update cmd/manager/main.go to call this cluster.Add(mgr) to install this Controller
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("nodegroup-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to NodeGroup
	err = c.Watch(&source.Kind{Type: &clusterv1alpha1.NodeGroup{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create
	// Uncomment watch a Deployment created by NodeGroup - change this for objects you create
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &clusterv1alpha1.NodeGroup{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileNodeGroup{}

var (
	StatusCreateComplete = "Create Node Group Complete"
	StatusCreateFailed   = "Create Node Group Failed"
	StatusDeleteFailed   = "Delete Node Group Failed"
	StatusError          = "Node Group Error"
	FinalizerCFNStack    = "cfn-stack.nodegroup.eks.amazonaws.com"
)

// Reconcile reads that state of the cluster for a NodeGroup object and makes changes based on the state read
// and what is in the NodeGroup.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  The scaffolding writes
// a Deployment as an example
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.eks.amazonaws.com,resources=nodegroups,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcileNodeGroup) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	logger := r.log.With(
		zap.String("Kind", "NodeGroup"),
		zap.String("Name", request.Name),
		zap.String("NameSpace", request.Namespace),
	)

	// Fetch the NodeGroup instance
	instance := &clusterv1alpha1.NodeGroup{}
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

	logger.Info("Got reconcile request for nodegroup")

	labelsToVerify := []string{"eks.owner.name", "eks.owner.namespace"}
	for _, label := range labelsToVerify {
		if _, ok := instance.Labels[label]; !ok {
			err := fmt.Errorf("%s label is missing from control plane", label)
			return reconcile.Result{}, r.setNodeGroupError(instance, err.Error(), err, logger)
		}
	}

	eksCluster := &clusterv1alpha1.EKS{}
	if err = r.Get(context.TODO(), types.NamespacedName{Name: instance.Labels["eks.owner.name"], Namespace: instance.Labels["eks.owner.namespace"]}, eksCluster); err != nil {
		return reconcile.Result{}, r.setNodeGroupError(instance, "EKS cluster not found", err, logger)
	}

	var cfnSvc cloudformationiface.CloudFormationAPI
	if r.cfnSvc == nil {
		targetAccountSession, err := eksCluster.Spec.GetCrossAccountSession(r.sess)
		if err != nil {
			return reconcile.Result{}, r.setNodeGroupError(instance, "Error getting the cross account session", err, logger)
		}

		cfnSvc = cloudformation.New(targetAccountSession)
	} else {
		cfnSvc = r.cfnSvc
	}

	if !instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if finalizers.HasFinalizer(instance, FinalizerCFNStack) {
			logger.Info("deleting nodegroup cloudformation stack", zap.String("AWSAccountID", eksCluster.Spec.AccountID), zap.String("AWSRegion", eksCluster.Spec.Region), zap.String("StackName", getCFNStackName(instance)))

			err = cfnhelper.DeleteStack(cfnSvc, getCFNStackName(instance))
			if err != nil {
				logger.Error("error deleting node group cloudformation stack", zap.Error(err))
				r.setNodeGroupStatus(StatusDeleteFailed, instance)
				return reconcile.Result{}, err
			}
			logger.Info("cloudformation stack deleted successfully", zap.String("StackName", getCFNStackName(instance)))

			instance.SetFinalizers(finalizers.RemoveFinalizer(instance, FinalizerCFNStack))
			err = r.Update(context.TODO(), instance)
			if err != nil {
				return reconcile.Result{}, err
			}
		}

		return reconcile.Result{Requeue: true}, nil
	}

	if instance.Status.Status == StatusCreateComplete || instance.Status.Status == StatusCreateFailed {
		// TODO : Need to implement Update Scenario
		return reconcile.Result{}, nil
	}

	logger.Info(fmt.Sprintf("Creating %s Node Group stack for %v account in %v", getCFNStackName(instance), eksCluster.Spec.AccountID, eksCluster.Spec.Region))

	if err = r.createNodeGroupStack(cfnSvc, instance, eksCluster); err != nil {
		r.setNodeGroupStatus(StatusCreateFailed, instance)
		logger.Error("Error creating the nodegroup stack", zap.Error(err))
		return reconcile.Result{}, err
	}

	logger.Info(fmt.Sprintf("Cloudformation stack created successfully %s", getCFNStackName(instance)))

	instance.SetFinalizers(finalizers.AddFinalizer(instance, FinalizerCFNStack))
	return reconcile.Result{}, r.setNodeGroupStatus(StatusCreateComplete, instance)
}

func (r *ReconcileNodeGroup) createNodeGroupStack(cfnSvc cloudformationiface.CloudFormationAPI, nodegroup *clusterv1alpha1.NodeGroup, eks *clusterv1alpha1.EKS) error {
	var eksOptimizedAMIs = map[string]string{
		"us-east-1": "ami-0440e4f6b9713faf6",
		"us-west-2": "ami-0a54c984b9f908c81",
		"eu-west-1": "ami-0c7a4976cb6fafd3a",
	}

	templateBody, err := cfnhelper.GetCFNTemplateBody(nodeGroupCFNTemplate, map[string]string{
		"ClusterName":           eks.Spec.ControlPlane.ClusterName,
		"ControlPlaneStackName": "eks-" + eks.Spec.ControlPlane.ClusterName,
		"AMI":                   eksOptimizedAMIs[eks.Spec.Region],
	})

	if err != nil {
		return err
	}

	_, err = cfnhelper.CreateAndDescribeStack(cfnSvc, &cloudformation.CreateStackInput{
		TemplateBody: aws.String(templateBody),
		StackName:    aws.String(getCFNStackName(nodegroup)),
		Capabilities: []*string{aws.String("CAPABILITY_IAM")},
		Tags: []*cloudformation.Tag{
			{
				Key:   aws.String("ClusterName"),
				Value: aws.String(eks.Spec.ControlPlane.ClusterName),
			},
		},
	})

	return err
}

func (r *ReconcileNodeGroup) setNodeGroupStatus(msg string, instance *clusterv1alpha1.NodeGroup) error {
	instance.Status.Status = msg
	return r.Update(context.TODO(), instance)
}

func (r *ReconcileNodeGroup) setNodeGroupError(instance *clusterv1alpha1.NodeGroup, errMsg string, err error, logger *zap.Logger) error {
	logger.Error(errMsg, zap.Error(err))
	if setStatusError := r.setNodeGroupStatus(StatusError, instance); setStatusError != nil {
		logger.Error("Error setting the status", zap.Error(setStatusError))
		return setStatusError
	}
	return err
}

func getCFNStackName(instance *clusterv1alpha1.NodeGroup) string {
	return fmt.Sprintf("eks-%s", instance.Spec.Name)
}
