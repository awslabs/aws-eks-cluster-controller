package controlplane

import (
	"fmt"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	clusterv1alpha1 "github.com/awslabs/aws-eks-cluster-controller/pkg/apis/cluster/v1alpha1"
	"github.com/awslabs/aws-eks-cluster-controller/pkg/logging"

	"github.com/awslabs/aws-eks-cluster-controller/pkg/apis"
	awsHelper "github.com/awslabs/aws-eks-cluster-controller/pkg/aws"
	fakeaws "github.com/awslabs/aws-eks-cluster-controller/pkg/aws"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/aws-sdk-go/aws/awserr"

	"github.com/onsi/gomega"
	"golang.org/x/net/context"
)

var errDoesNotExist = awserr.New("ValidationError", `ValidationError: Stack with id eks-foo-cluster does not exist, status code: 400, request id: 42`, nil)

func newTestReconciler(cfnSvc *awsHelper.MockCloudformationAPI, ns string, blankObjs ...metav1.Object) *ReconcileControlPlane {
	objs := []runtime.Object{}
	for _, obj := range blankObjs {
		obj.SetNamespace(ns)
		objs = append(objs, obj.(runtime.Object))
	}
	objs = append(objs, &clusterv1alpha1.EKS{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-eks-name",
			Namespace: "test-eks-namespace",
		},
		Spec: clusterv1alpha1.EKSSpec{
			ControlPlane: clusterv1alpha1.ControlPlaneSpec{
				ClusterName: "test-clustername",
			},
		},
	})

	if cfnSvc == nil {
		cfnSvc = &awsHelper.MockCloudformationAPI{
			FailDescribe: true,
			Err:          errDoesNotExist,
		}
	}
	return &ReconcileControlPlane{
		Client: fakeclient.NewFakeClient(objs...),
		scheme: scheme.Scheme,
		log:    logging.New(),
		cfnSvc: cfnSvc,
	}
}

func TestReconcile(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	tests := []struct {
		name       string
		cfnSvc     *awsHelper.MockCloudformationAPI
		request    reconcile.Request
		worldState []metav1.Object
		finalState *clusterv1alpha1.ControlPlane
		wantErr    bool
	}{
		{
			name: "updates instance status to error if missing labels",
			worldState: []metav1.Object{
				&clusterv1alpha1.ControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name: "uut",
					},
				},
			},
			finalState: &clusterv1alpha1.ControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "uut",
				},
				Status: clusterv1alpha1.ControlPlaneStatus{
					Status: "Error",
				},
			},
			wantErr: true,
		},
		{
			name: "updates instance status to error if missing EKS cluster",
			worldState: []metav1.Object{
				&clusterv1alpha1.ControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name: "uut",
						Labels: map[string]string{
							"eks.owner.name":      "test-eks-name",
							"eks.owner.namespace": "test-eks-doesnotexist",
						},
					},
				},
			},
			finalState: &clusterv1alpha1.ControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "uut",
				},
				Status: clusterv1alpha1.ControlPlaneStatus{
					Status: "Error",
				},
			},
			wantErr: true,
		},
		{
			name: "can describe a controlplane stack",
			worldState: []metav1.Object{
				&clusterv1alpha1.ControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name: "uut",
						Labels: map[string]string{
							"eks.owner.name":      "test-eks-name",
							"eks.owner.namespace": "test-eks-namespace",
						},
					},
				},
			},
			finalState: &clusterv1alpha1.ControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "uut",
				},
				Status: clusterv1alpha1.ControlPlaneStatus{
					Status: "Creating",
				},
			},
		},
		{
			name: "will complete when controlplane is finished",
			worldState: []metav1.Object{
				&clusterv1alpha1.ControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name: "uut",
						Labels: map[string]string{
							"eks.owner.name":      "test-eks-name",
							"eks.owner.namespace": "test-eks-namespace",
						},
					},
				},
			},
			finalState: &clusterv1alpha1.ControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "uut",
				},
				Status: clusterv1alpha1.ControlPlaneStatus{
					Status: "Complete",
				},
			},
			cfnSvc: &awsHelper.MockCloudformationAPI{Status: awsHelper.CompleteStatuses[0]},
		},
	}
	count := uint64(0)
	for _, tt := range tests {
		count := atomic.AddUint64(&count, 1)
		ns := fmt.Sprintf("test-%02d", count)
		func(ns string, count uint64) {
			t.Run(tt.name, func(t *testing.T) {
				reconciler := newTestReconciler(tt.cfnSvc, ns, tt.worldState...)

				_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "uut", Namespace: ns}})

				if (err != nil) != tt.wantErr {
					t.Errorf("ReconcileControlPlane.Reconcile() error = %v, wantErr %v", err, tt.wantErr)
					return
				}

				obj := &clusterv1alpha1.ControlPlane{}
				g.Expect(reconciler.Get(context.TODO(), client.ObjectKey{Name: "uut", Namespace: ns}, obj)).To(gomega.BeNil())

				g.Expect(obj.Spec).To(gomega.Equal(tt.finalState.Spec))
				g.Expect(obj.Status).To(gomega.Equal(tt.finalState.Status))

			})
		}(ns, count)
	}
}

func TestReconcileControlPlane_Reconcile_CloudformationStatusChecks(t *testing.T) {
	apis.AddToScheme(scheme.Scheme)
	tests := []struct {
		name     string
		statuses []string
		want     reconcile.Result
		wantErr  bool
	}{
		{
			name:     "requeues if stack has a pending status",
			statuses: awsHelper.PendingStatuses,
			want:     reconcile.Result{RequeueAfter: 5 * time.Second},
			wantErr:  false,
		},
		{
			name:     "completes if stack hsa a completed status",
			statuses: awsHelper.CompleteStatuses,
			want:     reconcile.Result{},
			wantErr:  false,
		},
		{
			name:     "fails if stack has a failed status",
			statuses: awsHelper.FailedStatuses,
			want:     reconcile.Result{},
			wantErr:  false,
		},
		{
			name:     "requeues if stack has an unexpected status",
			statuses: []string{"invalid status"},
			want:     reconcile.Result{RequeueAfter: 5 * time.Second},
			wantErr:  false,
		},
	}
	for _, tt := range tests {
		for _, status := range tt.statuses {
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
