package nodegroup

import (
	"fmt"
	"testing"
	"time"

	clusterv1alpha1 "github.com/awslabs/aws-eks-cluster-controller/pkg/apis/cluster/v1alpha1"
	"github.com/awslabs/aws-eks-cluster-controller/pkg/cfnhelper"
	"github.com/awslabs/aws-eks-cluster-controller/pkg/logging"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/onsi/gomega"
	"golang.org/x/net/context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var c client.Client

func getRequest(name string) reconcile.Request {
	return reconcile.Request{NamespacedName: types.NamespacedName{Name: name, Namespace: "default"}}
}
func getNGKey(name string) types.NamespacedName {
	return types.NamespacedName{Name: name, Namespace: "default"}
}

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
				clusterv1alpha1.NodeGroupSpec{Name: "NG1"},
			},
		},
	}
}

const timeout = time.Second * 25

func newTestReconciler(mgr manager.Manager, cfn *cfnhelper.MockCloudformationAPI) reconcile.Reconciler {
	return &ReconcileNodeGroup{
		Client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
		log:    logging.New(),
		sess:   nil,
		cfnSvc: cfn,
	}
}

func TestNodeGroupReconcile(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	type testCase struct {
		instance       *clusterv1alpha1.NodeGroup
		expectedStatus string
		logMessage     string
		testKey        string
		cfnSvc         *cfnhelper.MockCloudformationAPI
	}

	var testCases = []*testCase{}

	createIsSuccessful := new(testCase)
	createIsSuccessful.instance = &clusterv1alpha1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "foopla", Namespace: "default", Labels: map[string]string{"eks.owner.name": "eks-foopla", "eks.owner.namespace": "default"}},
		Spec: clusterv1alpha1.NodeGroupSpec{
			Name: "foopla-ngroup1",
		},
	}
	createIsSuccessful.expectedStatus = StatusCreateComplete
	createIsSuccessful.logMessage = "Create works correctly"
	createIsSuccessful.testKey = "foopla"
	createIsSuccessful.cfnSvc = &cfnhelper.MockCloudformationAPI{FailDescribe: true, ResetDescribe: true, Err: awserr.New("ValidationError", "Stack with id eks-foopla-ngroup1 does not exist", nil)}
	testCases = append(testCases, createIsSuccessful)

	//TODO Add Remaining TestCases

	for _, test := range testCases {
		mgr, err := manager.New(cfg, manager.Options{})
		g.Expect(err).NotTo(gomega.HaveOccurred())
		c = mgr.GetClient()

		cfnSvc := test.cfnSvc

		recFn, requests := SetupTestReconcile(newTestReconciler(mgr, cfnSvc))
		g.Expect(add(mgr, recFn)).NotTo(gomega.HaveOccurred())

		stopMgr, mgrStopped := StartTestManager(mgr, g)

		defer func() {
			close(stopMgr)
			mgrStopped.Wait()
		}()

		eksFooCluster := getEKSCluster(fmt.Sprintf("eks-%s", test.testKey))

		g.Expect(c.Create(context.TODO(), eksFooCluster)).NotTo(gomega.HaveOccurred())
		defer c.Delete(context.TODO(), eksFooCluster)

		err = c.Create(context.TODO(), test.instance)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		defer c.Delete(context.TODO(), test.instance)
		g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(getRequest(test.testKey))))

		g.Eventually(func() (string, error) {
			nodegroup := &clusterv1alpha1.NodeGroup{}
			err := c.Get(context.TODO(), getNGKey(test.testKey), nodegroup)
			return nodegroup.Status.Status, err
		}, timeout).Should(gomega.Equal(test.expectedStatus))

		t.Logf("Test Completed - %s", test.logMessage)
	}
}

func TestNodeGroupGetCFNStackNamereturnsExpectedName(t *testing.T) {
	instance := &clusterv1alpha1.NodeGroup{
		Spec: clusterv1alpha1.NodeGroupSpec{
			Name: "foo",
		},
	}
	actual := getCFNStackName(instance)
	expected := "eks-foo"
	if actual != expected {
		t.Errorf("getCFNStackname returned %s for foo", actual)
	}
}
