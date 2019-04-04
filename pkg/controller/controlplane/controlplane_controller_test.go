package controlplane

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	clusterv1alpha1 "github.com/awslabs/aws-eks-cluster-controller/pkg/apis/cluster/v1alpha1"
	"github.com/awslabs/aws-eks-cluster-controller/pkg/logging"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/awslabs/aws-eks-cluster-controller/pkg/apis"
	awsHelper "github.com/awslabs/aws-eks-cluster-controller/pkg/aws"
	fakeaws "github.com/awslabs/aws-eks-cluster-controller/pkg/aws"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/aws-sdk-go/service/cloudformation"

	"github.com/aws/aws-sdk-go/aws/awserr"

	"github.com/onsi/gomega"
	"golang.org/x/net/context"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const timeout = time.Second * 10

type cfnSvcConfig struct {
	FailCreate   bool
	FailDescribe bool
	FailDelete   bool
}

func newTestReconciler(mgr manager.Manager, cfnSvcConfig cfnSvcConfig) *ReconcileControlPlane {
	var errDoesNotExist = awserr.New("ValidationError", `ValidationError: Stack with id eks-foo-cluster does not exist, status code: 400, request id: 42`, nil)
	return &ReconcileControlPlane{
		Client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
		log:    logging.New(),
		cfnSvc: &awsHelper.MockCloudformationAPI{
			FailCreate:   cfnSvcConfig.FailCreate,
			FailDelete:   cfnSvcConfig.FailDelete,
			FailDescribe: cfnSvcConfig.FailDescribe,
			Err:          errDoesNotExist,
		},
	}
}

func TestReconcileSuccess(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	expectedRequest := reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo-cp", Namespace: "default"}}
	cpKey := types.NamespacedName{Name: "foo-cp", Namespace: "default"}

	cluster := &clusterv1alpha1.EKS{
		ObjectMeta: metav1.ObjectMeta{Name: "foo-eks", Namespace: "default"},
		Spec: clusterv1alpha1.EKSSpec{
			AccountID: "1234",
			ControlPlane: clusterv1alpha1.ControlPlaneSpec{
				ClusterName: "foo-cluster",
			},
			CrossAccountRoleName: "foo-role",
			NodeGroups:           []clusterv1alpha1.NodeGroupSpec{{Name: "foo-ng", IAMPolicies: []clusterv1alpha1.Policy{}}},
			Region:               "us-test-1",
		},
	}

	instance := &clusterv1alpha1.ControlPlane{
		ObjectMeta: metav1.ObjectMeta{Name: "foo-cp", Namespace: "default", Labels: map[string]string{"eks.owner.name": "foo-eks", "eks.owner.namespace": "default"}},
		Spec: clusterv1alpha1.ControlPlaneSpec{
			ClusterName: "foo-cluster",
		},
	}

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	c := mgr.GetClient()

	reconciler := newTestReconciler(mgr, cfnSvcConfig{FailDescribe: true})
	recFn, requests := SetupTestReconcile(reconciler)
	g.Expect(add(mgr, recFn)).NotTo(gomega.HaveOccurred())

	stopMgr, mgrStopped := StartTestManager(mgr, g)

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	g.Expect(c.Create(context.TODO(), cluster)).NotTo(gomega.HaveOccurred())
	defer c.Delete(context.TODO(), cluster)

	// Create the ControlPlane object and expect the Reconcile and Deployment to be created
	err = c.Create(context.TODO(), instance)
	if apierrors.IsInvalid(err) {
		t.Logf("failed to create object, got an invalid object error: %v", err)
		return
	}
	g.Expect(err).NotTo(gomega.HaveOccurred())

	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	getCP := &clusterv1alpha1.ControlPlane{}
	g.Eventually(func() (string, error) {
		err := c.Get(context.TODO(), cpKey, getCP)
		return getCP.Status.Status, err
	}).Should(gomega.Equal(StatusCreating))

	reconciler.cfnSvc = &awsHelper.MockCloudformationAPI{Status: cloudformation.StackStatusCreateComplete}
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	g.Eventually(func() (string, error) {
		err := c.Get(context.TODO(), cpKey, getCP)
		return getCP.Status.Status, err
	}).Should(gomega.Equal(StatusCreateComplete))

	err = c.Delete(context.TODO(), instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))
}

func TestReconcileFail(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	expectedRequest := reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-controlplane-name", Namespace: "test-controlplane-namespace"}}
	cpKey := types.NamespacedName{Name: "test-controlplane-name", Namespace: "test-controlplane-namespace"}

	cluster := &clusterv1alpha1.EKS{
		ObjectMeta: metav1.ObjectMeta{Name: "test-eks-name", Namespace: "test-controlplane-namespace"},
		Spec: clusterv1alpha1.EKSSpec{
			AccountID: "test-account-id",
			ControlPlane: clusterv1alpha1.ControlPlaneSpec{
				ClusterName: "test-cluster-name",
			},
			CrossAccountRoleName: "test-crossaccount-role",
			NodeGroups:           []clusterv1alpha1.NodeGroupSpec{{Name: "test-nodegroup-name", IAMPolicies: []clusterv1alpha1.Policy{}}},
			Region:               "test-region-id",
		},
	}

	instance := &clusterv1alpha1.ControlPlane{
		ObjectMeta: metav1.ObjectMeta{Name: "test-controlplane-name", Namespace: "test-controlplane-namespace", Labels: map[string]string{"eks.owner.name": "test-eks-name", "eks.owner.namespace": "test-controlplane-namespace"}},
		Spec: clusterv1alpha1.ControlPlaneSpec{
			ClusterName: "test-cluster-name",
		},
	}

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	c := mgr.GetClient()

	reconciler := newTestReconciler(mgr, cfnSvcConfig{FailDescribe: true})

	recFn, requests := SetupTestReconcile(reconciler)
	g.Expect(add(mgr, recFn)).NotTo(gomega.HaveOccurred())

	stopMgr, mgrStopped := StartTestManager(mgr, g)

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	g.Expect(c.Create(context.TODO(), cluster)).NotTo(gomega.HaveOccurred())
	defer c.Delete(context.TODO(), cluster)

	// Create the ControlPlane object and expect the Reconcile and Deployment to be created
	err = c.Create(context.TODO(), instance)
	if apierrors.IsInvalid(err) {
		t.Logf("failed to create object, got an invalid object error: %v", err)
		return
	}
	g.Expect(err).NotTo(gomega.HaveOccurred())

	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	getCP := &clusterv1alpha1.ControlPlane{}
	g.Eventually(func() (string, error) {
		err := c.Get(context.TODO(), cpKey, getCP)
		return getCP.Status.Status, err
	}).Should(gomega.Equal(StatusCreating))

	reconciler.cfnSvc = &awsHelper.MockCloudformationAPI{Status: cloudformation.StackStatusRollbackFailed}
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	g.Eventually(func() (string, error) {
		err := c.Get(context.TODO(), cpKey, getCP)
		return getCP.Status.Status, err
	}).Should(gomega.Equal(StatusFailed))

	// err = c.Delete(context.TODO(), instance)
	// g.Expect(err).NotTo(gomega.HaveOccurred())
	// g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))
}

func TestReconcileControlPlane_Reconcile(t *testing.T) {
	type fields struct {
		Client client.Client
		scheme *runtime.Scheme
		log    *zap.Logger
		sess   *session.Session
		cfnSvc cloudformationiface.CloudFormationAPI
	}
	type args struct {
		request reconcile.Request
	}
	apis.AddToScheme(scheme.Scheme)
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    reconcile.Result
		wantErr bool
	}{
		{
			name: "detects is controlplane is missing",
			fields: fields{
				Client: fakeclient.NewFakeClient(),
				scheme: scheme.Scheme,
				log:    logging.New(),
			},
			args: args{
				request: reconcile.Request{
					NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"},
				},
			},
			want:    reconcile.Result{},
			wantErr: false,
		},
		{
			name: "updates instance status to error if missing labels",
			fields: fields{
				Client: fakeclient.NewFakeClient(&clusterv1alpha1.ControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-name",
						Namespace: "test-namespace",
					},
				}),
				scheme: scheme.Scheme,
				log:    logging.New(),
			},
			args: args{
				request: reconcile.Request{
					NamespacedName: types.NamespacedName{Name: "test-name", Namespace: "test-namespace"},
				},
			},
			want:    reconcile.Result{},
			wantErr: true,
		},
		{
			name: "updates instance status to error if missing EKS cluster",
			fields: fields{
				Client: fakeclient.NewFakeClient(
					&clusterv1alpha1.ControlPlane{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-controlplane-name",
							Namespace: "test-controlplane-namespace",
							Labels: map[string]string{
								"eks.owner.name":      "test-eks-name",
								"eks.owner.namespace": "test-eks-namespace",
							},
						},
					},
				),
				scheme: scheme.Scheme,
				log:    logging.New(),
			},
			args: args{
				request: reconcile.Request{
					NamespacedName: types.NamespacedName{Name: "test-controlplane-name", Namespace: "test-controlplane-namespace"},
				},
			},
			want:    reconcile.Result{},
			wantErr: true,
		},
		{
			name: "can describe a controlplane stack",
			fields: fields{
				Client: fakeclient.NewFakeClient(
					&clusterv1alpha1.ControlPlane{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-controlplane-name",
							Namespace: "test-controlplane-namespace",
							Labels: map[string]string{
								"eks.owner.name":      "test-eks-name",
								"eks.owner.namespace": "test-eks-namespace",
							},
						},
					},
					&clusterv1alpha1.EKS{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-eks-name",
							Namespace: "test-eks-namespace",
						},
						Spec: clusterv1alpha1.EKSSpec{
							ControlPlane: clusterv1alpha1.ControlPlaneSpec{
								ClusterName: "test-clustername",
							},
						},
					},
				),
				scheme: scheme.Scheme,
				log:    logging.New(),
				cfnSvc: &fakeaws.MockCloudformationAPI{},
			},
			args: args{
				request: reconcile.Request{
					NamespacedName: types.NamespacedName{Name: "test-controlplane-name", Namespace: "test-controlplane-namespace"},
				},
			},
			want:    reconcile.Result{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ReconcileControlPlane{
				Client: tt.fields.Client,
				scheme: tt.fields.scheme,
				log:    tt.fields.log,
				sess:   tt.fields.sess,
				cfnSvc: tt.fields.cfnSvc,
			}
			got, err := r.Reconcile(tt.args.request)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReconcileControlPlane.Reconcile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ReconcileControlPlane.Reconcile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReconcileControlPlane_Reconcile_CloudformationStatusPending(t *testing.T) {
	apis.AddToScheme(scheme.Scheme)
	tests := []struct {
		statuses []string
		want     reconcile.Result
		wantErr  bool
	}{
		{
			statuses: awsHelper.PendingStatuses,
			want:     reconcile.Result{RequeueAfter: 5 * time.Second},
			wantErr:  false,
		},
	}
	for _, tt := range tests {
		for _, status := range tt.statuses {
			name := fmt.Sprintf("requeues if cloudformation status is %s", status)
			t.Run(name, func(t *testing.T) {
				r := &ReconcileControlPlane{
					Client: fakeclient.NewFakeClient(
						&clusterv1alpha1.ControlPlane{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "test-controlplane-name",
								Namespace: "test-controlplane-namespace",
								Labels: map[string]string{
									"eks.owner.name":      "test-eks-name",
									"eks.owner.namespace": "test-eks-namespace",
								},
							},
						},
						&clusterv1alpha1.EKS{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "test-eks-name",
								Namespace: "test-eks-namespace",
							},
							Spec: clusterv1alpha1.EKSSpec{
								ControlPlane: clusterv1alpha1.ControlPlaneSpec{
									ClusterName: "test-clustername",
								},
							},
						},
					),
					scheme: scheme.Scheme,
					log:    logging.New(),
					cfnSvc: &fakeaws.MockCloudformationAPI{Status: status},
				}
				got, err := r.Reconcile(
					reconcile.Request{
						NamespacedName: types.NamespacedName{Name: "test-controlplane-name", Namespace: "test-controlplane-namespace"},
					},
				)
				if (err != nil) != tt.wantErr {
					t.Errorf("ReconcileControlPlane.Reconcile() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("ReconcileControlPlane.Reconcile() = %v, want %v", got, tt.want)
				}
			})
		}
	}
}

func TestReconcileControlPlane_Reconcile_CloudformationStatusComplete(t *testing.T) {
	apis.AddToScheme(scheme.Scheme)
	tests := []struct {
		statuses []string
		want     reconcile.Result
		wantErr  bool
	}{
		{
			statuses: awsHelper.CompleteStatuses,
			want:     reconcile.Result{},
			wantErr:  false,
		},
	}
	for _, tt := range tests {
		for _, status := range tt.statuses {
			name := fmt.Sprintf("requeues if cloudformation status is %s", status)
			t.Run(name, func(t *testing.T) {
				r := &ReconcileControlPlane{
					Client: fakeclient.NewFakeClient(
						&clusterv1alpha1.ControlPlane{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "test-controlplane-name",
								Namespace: "test-controlplane-namespace",
								Labels: map[string]string{
									"eks.owner.name":      "test-eks-name",
									"eks.owner.namespace": "test-eks-namespace",
								},
							},
						},
						&clusterv1alpha1.EKS{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "test-eks-name",
								Namespace: "test-eks-namespace",
							},
							Spec: clusterv1alpha1.EKSSpec{
								ControlPlane: clusterv1alpha1.ControlPlaneSpec{
									ClusterName: "test-clustername",
								},
							},
						},
					),
					scheme: scheme.Scheme,
					log:    logging.New(),
					cfnSvc: &fakeaws.MockCloudformationAPI{Status: status},
				}
				got, err := r.Reconcile(
					reconcile.Request{
						NamespacedName: types.NamespacedName{Name: "test-controlplane-name", Namespace: "test-controlplane-namespace"},
					},
				)
				if (err != nil) != tt.wantErr {
					t.Errorf("ReconcileControlPlane.Reconcile() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("ReconcileControlPlane.Reconcile() = %v, want %v", got, tt.want)
				}
			})
		}
	}
}

func TestReconcileControlPlane_Reconcile_CloudformationStatusFailed(t *testing.T) {
	apis.AddToScheme(scheme.Scheme)
	tests := []struct {
		statuses []string
		want     reconcile.Result
		wantErr  bool
	}{
		{
			statuses: awsHelper.FailedStatuses,
			want:     reconcile.Result{},
			wantErr:  false,
		},
	}
	for _, tt := range tests {
		for _, status := range tt.statuses {
			name := fmt.Sprintf("requeues if cloudformation status is %s", status)
			t.Run(name, func(t *testing.T) {
				r := &ReconcileControlPlane{
					Client: fakeclient.NewFakeClient(
						&clusterv1alpha1.ControlPlane{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "test-controlplane-name",
								Namespace: "test-controlplane-namespace",
								Labels: map[string]string{
									"eks.owner.name":      "test-eks-name",
									"eks.owner.namespace": "test-eks-namespace",
								},
							},
						},
						&clusterv1alpha1.EKS{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "test-eks-name",
								Namespace: "test-eks-namespace",
							},
							Spec: clusterv1alpha1.EKSSpec{
								ControlPlane: clusterv1alpha1.ControlPlaneSpec{
									ClusterName: "test-clustername",
								},
							},
						},
					),
					scheme: scheme.Scheme,
					log:    logging.New(),
					cfnSvc: &fakeaws.MockCloudformationAPI{Status: status},
				}
				got, err := r.Reconcile(
					reconcile.Request{
						NamespacedName: types.NamespacedName{Name: "test-controlplane-name", Namespace: "test-controlplane-namespace"},
					},
				)
				if (err != nil) != tt.wantErr {
					t.Errorf("ReconcileControlPlane.Reconcile() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("ReconcileControlPlane.Reconcile() = %v, want %v", got, tt.want)
				}
			})
		}
	}
}

func TestReconcileControlPlane_Reconcile_CloudformationStatusUnexpected(t *testing.T) {
	apis.AddToScheme(scheme.Scheme)
	tests := []struct {
		name      string
		cfnStatus string
		want      reconcile.Result
		wantErr   bool
	}{
		{
			name:      "errors if cloudformation stack in unexpected state",
			cfnStatus: "invalid status",
			want:      reconcile.Result{},
			wantErr:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ReconcileControlPlane{
				Client: fakeclient.NewFakeClient(
					&clusterv1alpha1.ControlPlane{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-controlplane-name",
							Namespace: "test-controlplane-namespace",
							Labels: map[string]string{
								"eks.owner.name":      "test-eks-name",
								"eks.owner.namespace": "test-eks-namespace",
							},
						},
					},
					&clusterv1alpha1.EKS{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-eks-name",
							Namespace: "test-eks-namespace",
						},
						Spec: clusterv1alpha1.EKSSpec{
							ControlPlane: clusterv1alpha1.ControlPlaneSpec{
								ClusterName: "test-clustername",
							},
						},
					},
				),
				scheme: scheme.Scheme,
				log:    logging.New(),
				cfnSvc: &fakeaws.MockCloudformationAPI{Status: tt.cfnStatus},
			}
			got, err := r.Reconcile(reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "test-controlplane-name", Namespace: "test-controlplane-namespace"},
			},
			)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReconcileControlPlane.Reconcile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ReconcileControlPlane.Reconcile() = %v, want %v", got, tt.want)
			}
		})
	}
}
