package nodegroup

import (
	"context"
	"fmt"
	"time"

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
	c, err := controller.New("nodegroup-controller", mgr, controller.Options{Reconciler: r, MaxConcurrentReconciles: 5})
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
	StatusCreateComplete = "Complete"
	StatusCreating       = "Creating"
	StatusFailed         = "Failed"
	StatusError          = "Error"

	FinalizerCFNStack = "cfn-stack.nodegroup.eks.amazonaws.com"
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

	stackName := instance.Name

	labelsToVerify := []string{"eks.owner.name", "eks.owner.namespace"}
	for _, label := range labelsToVerify {
		if _, ok := instance.Labels[label]; !ok {
			logger.Error("label is missing", zap.String("LabelKey", label))
			instance.Status.Status = StatusError
			r.Update(context.TODO(), instance)
			return reconcile.Result{}, fmt.Errorf("%s label is missing from nodegroup", label)
		}
	}

	eksCluster := &clusterv1alpha1.EKS{}
	if err = r.Get(context.TODO(), types.NamespacedName{Name: instance.Labels["eks.owner.name"], Namespace: instance.Labels["eks.owner.namespace"]}, eksCluster); err != nil {
		logger.Error("EKS cluster not found", zap.Error(err))
		instance.Status.Status = StatusError
		r.Update(context.TODO(), instance)
		return reconcile.Result{}, err
	}

	var cfnSvc cloudformationiface.CloudFormationAPI
	if r.cfnSvc == nil {
		targetAccountSession, err := eksCluster.Spec.GetCrossAccountSession(r.sess)
		if err != nil {
			logger.Error("failed to get cross account session", zap.Error(err))
			instance.Status.Status = StatusError
			r.Update(context.TODO(), instance)
			return reconcile.Result{}, err
		}

		cfnSvc = cloudformation.New(targetAccountSession)
	} else {
		cfnSvc = r.cfnSvc
	}

	logger = logger.With(
		zap.String("AWSAccountID", eksCluster.Spec.AccountID),
		zap.String("AWSRegion", eksCluster.Spec.Region),
		zap.String("StackName", stackName),
	)

	if !instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if finalizers.HasFinalizer(instance, FinalizerCFNStack) {
			logger.Info("deleting nodegroup cloudformation stack")

			stack, err := cfnhelper.DescribeStack(cfnSvc, stackName)
			if err != nil && cfnhelper.IsDoesNotExist(err, stackName) {
				instance.SetFinalizers(finalizers.RemoveFinalizer(instance, FinalizerCFNStack))
				return reconcile.Result{}, r.Update(context.TODO(), instance)
			}
			if err != nil {
				r.fail(instance, "error deleting nodegroup cloudformation stack", err, logger)
				return reconcile.Result{}, err
			}
			if *stack.StackStatus == cloudformation.StackStatusDeleteComplete {
				instance.SetFinalizers(finalizers.RemoveFinalizer(instance, FinalizerCFNStack))
				return reconcile.Result{}, r.Update(context.TODO(), instance)
			}

			if *stack.StackStatus == cloudformation.StackStatusDeleteInProgress {
				return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
			}

			_, err = cfnSvc.DeleteStack(&cloudformation.DeleteStackInput{
				StackName: aws.String(stackName),
			})
			if err != nil {
				r.fail(instance, "error deleting nodegroup cloudformation stack", err, logger)
				return reconcile.Result{}, err
			}
			return reconcile.Result{Requeue: true}, nil
		}
	}

	stack, err := cfnhelper.DescribeStack(cfnSvc, stackName)
	if err != nil && cfnhelper.IsDoesNotExist(err, stackName) {
		logger.Info("creating nodegroup cloudformation stack")

		err = r.createNodeGroupStack(cfnSvc, instance, eksCluster)
		if err != nil {
			r.fail(instance, "error creating nodegroup cloudformation stack", err, logger)
			return reconcile.Result{}, err
		}

		logger.Info("cloudformation stack created successfully", zap.String("StackName", stackName))
		instance.Status.Status = StatusCreating
		instance.SetFinalizers(finalizers.AddFinalizer(instance, FinalizerCFNStack))
		err = r.Update(context.TODO(), instance)
		if err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		r.fail(instance, "error describing stack", err, logger)
		return reconcile.Result{}, err
	}

	// TODO: Add parameters to obj, and update if parameters change.

	logger.Info("Found Stack", zap.String("StackStatus", *stack.StackStatus))

	if *stack.StackStatus == cloudformation.StackStatusCreateFailed ||
		*stack.StackStatus == cloudformation.StackStatusRollbackFailed ||
		*stack.StackStatus == cloudformation.StackStatusUpdateRollbackFailed {
		instance.Status.Status = StatusFailed
		return reconcile.Result{}, r.Update(context.TODO(), instance)
	}

	if *stack.StackStatus != cloudformation.StackStatusCreateComplete &&
		*stack.StackStatus != cloudformation.StackStatusUpdateComplete &&
		*stack.StackStatus != cloudformation.StackStatusRollbackComplete &&
		*stack.StackStatus != cloudformation.StackStatusUpdateRollbackComplete {
		// Stack isn't done, wait longer.
		logger.Info("Stack not Complete requeueing")
		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
	}

	logger.Info("Stack Creation Complete")
	instance.Status.Status = StatusCreateComplete
	return reconcile.Result{}, r.Update(context.TODO(), instance)
}

func (r *ReconcileNodeGroup) fail(instance *clusterv1alpha1.NodeGroup, msg string, err error, logger *zap.Logger) {
	logger.Error(msg, zap.Error(err))
	instance.Status.Status = StatusFailed
	r.Update(context.TODO(), instance)
}

type nodeGroupTemplateInput struct {
	ClusterName           string
	ControlPlaneStackName string
	AMI                   string
	NodeInstanceName      string
	IAMPolicies           []clusterv1alpha1.Policy
}

func (r *ReconcileNodeGroup) createNodeGroupStack(cfnSvc cloudformationiface.CloudFormationAPI, nodegroup *clusterv1alpha1.NodeGroup, eks *clusterv1alpha1.EKS) error {

	// These AMIs are found https://docs.aws.amazon.com/eks/latest/userguide/eks-optimized-ami.html
	var eksOptimizedAMIs = map[string]string{
		"v1.11-us-west-2":  "ami-094fa4044a2a3cf52",
		"v1.11-us-east-1":  "ami-0b4eb1d8782fc3aea",
		"v1.11-us-east-2":  "ami-053cbe66e0033ebcf",
		"v1.11-eu-west-1":  "ami-0a9006fb385703b54",
		"v1.11-eu-north-1": "ami-082e6cf1c07e60241",
		"v1.10-us-west-2":  "ami-07af9511082779ae7",
		"v1.10-us-east-1":  "ami-027792c3cc6de7b5b",
		"v1.10-us-east-2":  "ami-036130f4127a367f7",
		"v1.10-eu-west-1":  "ami-03612357ac9da2c7d",
		"v1.10-eu-north-1": "ami-04b0f84e5a05e0b30",
	}

	templateBody, err := cfnhelper.GetCFNTemplateBody(nodeGroupCFNTemplate, nodeGroupTemplateInput{
		ClusterName:           eks.Spec.ControlPlane.ClusterName,
		ControlPlaneStackName: eks.GetControlPlaneStackName(),
		AMI:                   GetAMI(nodegroup.GetVersion(), eks.Spec.Region),
		NodeInstanceName:      nodegroup.Name,
		IAMPolicies:           nodegroup.Spec.IAMPolicies,
	})

	if err != nil {
		return err
	}

	_, err = cfnSvc.CreateStack(&cloudformation.CreateStackInput{
		TemplateBody: aws.String(templateBody),
		StackName:    aws.String(nodegroup.Name),
		Capabilities: []*string{aws.String("CAPABILITY_NAMED_IAM"), aws.String("CAPABILITY_IAM")},
		Tags: []*cloudformation.Tag{
			{
				Key:   aws.String("ClusterName"),
				Value: aws.String(eks.Spec.ControlPlane.ClusterName),
			},
		},
	})

	return err
}
