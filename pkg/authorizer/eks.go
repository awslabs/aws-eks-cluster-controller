package authorizer

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eks"

	"github.com/aws/aws-sdk-go/aws/session"
	clusterv1alpha1 "github.com/awslabs/aws-eks-cluster-controller/pkg/apis/cluster/v1alpha1"
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type EKSAuthorizer struct {
	rootSession *session.Session
	eksSvc      eksiface.EKSAPI
	log         *zap.Logger
}

func NewEks(rootSession *session.Session, log *zap.Logger) *EKSAuthorizer {
	return &EKSAuthorizer{
		rootSession: rootSession,
		log:         log,
	}
}

func (e *EKSAuthorizer) GetKubeConfig(eksCluster *clusterv1alpha1.EKS) ([]byte, error) {
	sess, err := eksCluster.Spec.GetCrossAccountSession(e.rootSession)
	if err != nil {
		e.log.Error("failed to get cross account session", zap.Error(err))
		return nil, err
	}
	eksSvc := e.eksSvc
	if eksSvc == nil {
		eksSvc = eks.New(sess)
	}

	roleARN := fmt.Sprintf("arn:aws:iam::%s:role/%s", eksCluster.Spec.AccountID, eksCluster.Spec.CrossAccountRoleName)
	return e.buildKubeconfig(eksSvc, eksCluster.Spec.ControlPlane.ClusterName, roleARN)
}

func (e *EKSAuthorizer) GetClient(eksCluster *clusterv1alpha1.EKS) (client.Client, error) {
	kconfig, err := e.GetKubeConfig(eksCluster)
	if err != nil {
		e.log.Error("failed to build kubconfig", zap.Error(err))
		return nil, err
	}

	return e.GetClientFromConfig(kconfig)
}

func (e *EKSAuthorizer) GetClientFromConfig(config []byte) (client.Client, error) {
	// Add our custom resources to the client
	s := scheme.Scheme

	cc, err := clientcmd.NewClientConfigFromBytes(config)
	if err != nil {
		e.log.Error("failed to build the client config", zap.Error(err))
		return nil, err
	}
	restConfig, err := cc.ClientConfig()
	if err != nil {
		e.log.Error("failed to build the rest config", zap.Error(err))
		return nil, err
	}
	kClient, err := client.New(restConfig, client.Options{Scheme: s})
	if err != nil {
		e.log.Error("failed to build the kubernetes client", zap.Error(err))
		return nil, err
	}

	return kClient, nil
}

var _ Authorizer = &EKSAuthorizer{}

type templateData struct {
	Name                     string
	Endpoint                 string
	CertificateAuthorityData string
	RoleARN                  string
}

func (e *EKSAuthorizer) buildKubeconfig(eksSvc eksiface.EKSAPI, clusterName, roleArn string) ([]byte, error) {
	log := e.log.With(zap.String("ClusterName", clusterName))

	r, err := eksSvc.DescribeCluster(&eks.DescribeClusterInput{
		Name: aws.String(clusterName),
	})
	if err != nil {
		log.Error("failed to describe the cluster", zap.Error(err))
		return nil, err
	}

	kubeconfigTemplate, err := template.New("kubeconfig").Parse(kubeconfig)
	if err != nil {
		log.Error("failed to parse the kubeconfig", zap.Error(err))
		return nil, err
	}

	data := templateData{
		Name:                     *r.Cluster.Name,
		Endpoint:                 *r.Cluster.Endpoint,
		CertificateAuthorityData: *r.Cluster.CertificateAuthority.Data,
		RoleARN:                  roleArn,
	}

	b := bytes.NewBuffer([]byte{})
	if err := kubeconfigTemplate.Execute(b, data); err != nil {
		log.Error("failed to apply the template", zap.Error(err))
		return nil, err
	}

	return b.Bytes(), nil
}

var kubeconfig = `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: {{ .CertificateAuthorityData }}
    server: {{ .Endpoint }}
  name: {{ .Name }}
contexts:
- context:
    cluster: {{ .Name }}
    user: aws-eks-cluster-operator@{{ .Name }}
  name: aws-eks-cluster-operator@{{ .Name }}
current-context: aws-eks-cluster-operator@{{ .Name }}
kind: Config
preferences: {}
users:
- name: aws-eks-cluster-operator@{{ .Name }}
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1alpha1
      args:
      - token
      - -i
      - {{ .Name }}
      - -r
      - {{ .RoleARN }}
      command: aws-iam-authenticator
      env: null
`
