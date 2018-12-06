package v1alpha1

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// EKSSpec defines the desired state of EKS
type EKSSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	AccountID            string           `json:"accountId"`
	CrossAccountRoleName string           `json:"crossAccountRoleName"`
	Region               string           `json:"region"`
	ControlPlane         ControlPlaneSpec `json:"controlPlane"`
	NodeGroups           []NodeGroupSpec  `json:"nodeGroups"`
}

// EKSStatus defines the observed state of EKS
type EKSStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Status       string         `json:"status"`
	ControlPlane *ControlPlane  `json:"controlPlane,omitempty"`
	NodeGroups   *NodeGroupList `json:"nodeGroups,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EKS is the Schema for the eks API
// +k8s:openapi-gen=true
type EKS struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EKSSpec   `json:"spec,omitempty"`
	Status EKSStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EKSList contains a list of EKS
type EKSList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EKS `json:"items"`
}

func init() {
	SchemeBuilder.Register(&EKS{}, &EKSList{})
}

func (e EKS) GetControlPlanes(c client.Client) (*ControlPlane, error) {
	cp := &ControlPlane{}
	cpKey := types.NamespacedName{Namespace: e.Namespace, Name: e.Name + "-controlplane"}

	err := c.Get(context.TODO(), cpKey, cp)
	return cp, err
}

func (e EKSSpec) GetCrossAccountSession(rootSession *session.Session) (*session.Session, error) {
	return session.NewSession(&aws.Config{
		Region:      aws.String(e.Region),
		Credentials: stscreds.NewCredentials(rootSession, fmt.Sprintf("arn:aws:iam::%s:role/%s", e.AccountID, e.CrossAccountRoleName)),
	})
}

func (e EKS) GetAWSAuthData() string {
	authString := "- groups:\n  - system:bootstrappers\n  - system:nodes\n  rolearn: %s\n  username: system:node:{{EC2PrivateDNSName}}\n"
	if len(e.Spec.NodeGroups) == 0 {
		return ""
	}
	auths := []string{}
	for _, ngName := range e.GetNodegroupNames() {

		auth := fmt.Sprintf(authString, getRoleARN(e.Spec.AccountID, ngName))
		auths = append(auths, auth)
	}
	return strings.Join(auths, "")

}
func (e EKS) GetNodegroupNames() []string {
	names := make([]string, 0, len(e.Spec.NodeGroups))
	for _, ng := range e.Spec.NodeGroups {
		names = append(names, strings.ToLower(fmt.Sprintf("%s-nodegroup-%s", e.Name, ng.Name)))
	}
	return names
}

func getRoleARN(account, name string) string {
	return fmt.Sprintf("arn:aws:iam::%s:role/%s-role", account, name)
}
