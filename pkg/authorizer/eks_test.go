package authorizer

import (
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eks"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	clusterv1alpha1 "github.com/awslabs/aws-eks-cluster-controller/pkg/apis/cluster/v1alpha1"
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestEKSAuthorizer_GetKubeConfig(t *testing.T) {
	type fields struct {
		rootSession *session.Session
		eksSvc      eksiface.EKSAPI
	}
	type args struct {
		eksCluster *clusterv1alpha1.EKS
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []byte
		wantErr bool
	}{
		{
			fields: fields{
				rootSession: session.New(),
				eksSvc:      fakeEKSSvc{},
			},
			args: args{eksCluster: &clusterv1alpha1.EKS{
				Spec: clusterv1alpha1.EKSSpec{
					AccountID:            "account1",
					CrossAccountRoleName: "stuff",
					Region:               "us-test-1",
				},
			}},
			want: []byte(wantKubeConfig),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &EKSAuthorizer{
				rootSession: tt.fields.rootSession,
				eksSvc:      tt.fields.eksSvc,
				log:         zap.NewExample(),
			}
			got, err := e.GetKubeConfig(tt.args.eksCluster)
			if (err != nil) != tt.wantErr {
				t.Errorf("EKSAuthorizer.GetKubeConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("EKSAuthorizer.GetKubeConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEKSAuthorizer_GetClient(t *testing.T) {
	type fields struct {
		rootSession *session.Session
		log         *zap.Logger
	}
	type args struct {
		eksCluster *clusterv1alpha1.EKS
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    client.Client
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &EKSAuthorizer{
				rootSession: tt.fields.rootSession,
				log:         tt.fields.log,
			}
			got, err := e.GetClient(tt.args.eksCluster)
			if (err != nil) != tt.wantErr {
				t.Errorf("EKSAuthorizer.GetClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("EKSAuthorizer.GetClient() = %v, want %v", got, tt.want)
			}
		})
	}
}

type fakeEKSSvc struct {
	eksiface.EKSAPI
}

func (s fakeEKSSvc) DescribeCluster(*eks.DescribeClusterInput) (*eks.DescribeClusterOutput, error) {
	return &eks.DescribeClusterOutput{
		Cluster: &eks.Cluster{
			CertificateAuthority: &eks.Certificate{Data: aws.String("CA1234")},
			Endpoint:             aws.String("endpoint-foo1"),
			Name:                 aws.String("Name1"),
		},
	}, nil
}

func Test_buildKubeconfig(t *testing.T) {

	type args struct {
		eksSvc      eksiface.EKSAPI
		clusterName string
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		{
			args: args{
				eksSvc:      fakeEKSSvc{},
				clusterName: "stuff",
			},
			want: []byte(wantKubeConfig),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := &EKSAuthorizer{
				log: zap.NewExample(),
			}
			got, err := auth.buildKubeconfig(tt.args.eksSvc, tt.args.clusterName, "roleArn")
			if (err != nil) != tt.wantErr {
				t.Errorf("buildKubeconfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildKubeconfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

var wantKubeConfig = `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: CA1234
    server: endpoint-foo1
  name: Name1
contexts:
- context:
    cluster: Name1
    user: aws-eks-cluster-operator@Name1
  name: aws-eks-cluster-operator@Name1
current-context: aws-eks-cluster-operator@Name1
kind: Config
preferences: {}
users:
- name: aws-eks-cluster-operator@Name1
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1alpha1
      args:
      - token
      - -i
      - Name1
      - -r
      - roleArn
      command: aws-iam-authenticator
      env: null
`
