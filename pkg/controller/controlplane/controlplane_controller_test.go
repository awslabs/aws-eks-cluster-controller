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
)

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
