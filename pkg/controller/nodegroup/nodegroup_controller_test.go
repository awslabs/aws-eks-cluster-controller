package nodegroup

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/awslabs/aws-eks-cluster-controller/pkg/apis/cluster/v1alpha1"
	clusterv1alpha1 "github.com/awslabs/aws-eks-cluster-controller/pkg/apis/cluster/v1alpha1"
	"github.com/awslabs/aws-eks-cluster-controller/pkg/cfnhelper"
	"github.com/awslabs/aws-eks-cluster-controller/pkg/logging"
	"github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var c client.Client

func getEKSCluster(name string) *clusterv1alpha1.EKS {
	return &clusterv1alpha1.EKS{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: clusterv1alpha1.EKSSpec{
			AccountID:            "1234foo",
			CrossAccountRoleName: "foo-role",
			Region:               "us-test-1",
			ControlPlane: clusterv1alpha1.ControlPlaneSpec{
				ClusterName: "cluster-stuff",
			},
			NodeGroups: []clusterv1alpha1.NodeGroupSpec{
				clusterv1alpha1.NodeGroupSpec{Name: "NG1", IAMPolicies: []v1alpha1.Policy{}},
			},
		},
	}
}

const timeout = time.Second * 5

func newTestReconciler(mgr manager.Manager) *ReconcileNodeGroup {
	var errDoesNotExist = awserr.New("ValidationError", "Stack with id eks-foopla-nodegroup-ngroup1 does not exist", nil)
	return &ReconcileNodeGroup{
		Client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
		log:    logging.New(),
		sess:   nil,
		cfnSvc: &cfnhelper.MockCloudformationAPI{FailDescribe: true, Err: errDoesNotExist},
	}
}

var expectedRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: "eks-foopla-nodegroup-ngroup1", Namespace: "default"}}
var ngKey = types.NamespacedName{Name: "eks-foopla-nodegroup-ngroup1", Namespace: "default"}

func TestNodeGroupReconcile(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	instance := &clusterv1alpha1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eks-foopla-nodegroup-ngroup1",
			Namespace: "default",
			Labels:    map[string]string{"eks.owner.name": "eks-foopla", "eks.owner.namespace": "default"},
		},
		Spec: clusterv1alpha1.NodeGroupSpec{
			Name:        "ngroup1",
			IAMPolicies: []v1alpha1.Policy{},
		},
	}

	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	c = mgr.GetClient()

	reconciler := newTestReconciler(mgr)
	recFn, requests := SetupTestReconcile(reconciler)
	g.Expect(add(mgr, recFn)).NotTo(gomega.HaveOccurred())

	stopMgr, mgrStopped := StartTestManager(mgr, g)

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	eksFooCluster := getEKSCluster("eks-foopla")

	g.Expect(c.Create(context.TODO(), eksFooCluster)).NotTo(gomega.HaveOccurred())
	defer c.Delete(context.TODO(), eksFooCluster)

	err = c.Create(context.TODO(), instance)
	if apierrors.IsInvalid(err) {
		t.Logf("failed to create object, got an invalid object error: %v", err)
		return
	}
	g.Expect(err).NotTo(gomega.HaveOccurred())

	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	getNG := &clusterv1alpha1.NodeGroup{}
	g.Eventually(func() (string, error) {
		err := c.Get(context.TODO(), ngKey, getNG)
		return getNG.Status.Status, err
	}).Should(gomega.Equal(StatusCreating))

	reconciler.cfnSvc = &cfnhelper.MockCloudformationAPI{Status: cloudformation.StackStatusCreateInProgress}
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	getNG = &clusterv1alpha1.NodeGroup{}
	err = c.Get(context.TODO(), ngKey, getNG)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(getNG.Status.Status).Should(gomega.Equal(StatusCreating))

	reconciler.cfnSvc = &cfnhelper.MockCloudformationAPI{Status: cloudformation.StackStatusCreateComplete}
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	g.Eventually(func() (string, error) {
		nodegroup := &clusterv1alpha1.NodeGroup{}
		err := c.Get(context.TODO(), ngKey, nodegroup)
		return nodegroup.Status.Status, err
	}, timeout).Should(gomega.Equal(StatusCreateComplete))

	err = c.Delete(context.TODO(), instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

}
