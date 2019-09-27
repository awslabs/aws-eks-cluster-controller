package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// NodeGroupSpec defines the desired state of NodeGroup
type NodeGroupSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Name string `json:"name"`
	// +optional
	Version *string `json:"version,omitempty"`
	// +optional
	IAMPolicies []Policy `json:"iamPolicies,omitempty"`
	// +optional
	Instance Instance `json:"instance,omitempty"`
}

// Policy represents an IAM Policy
type Policy struct {
	PolicyName     string         `json:"policyName"`
	PolicyDocument PolicyDocument `json:"policyDocument"`
}

// PolicyDocument represents an IAM PolicyDocument
type PolicyDocument struct {
	Version   string      `json:"version"`
	Statement []Statement `json:"statement"`
}

// Statement represents an IAM PolicyDocument Statement
type Statement struct {
	Effect   string   `json:"effect"`
	Action   []string `json:"action"`
	Resource []string `json:"resource"`
}

// Instance defines the instance details of the nodegroup
type Instance struct {
	// Default value is m5.large
	// +kubebuilder:validation:Enum=t2.small,t2.medium,t2.large,t2.xlarge,t2.2xlarge,m3.medium,m3.large,m3.xlarge,m3.2xlarge,m4.large,m4.xlarge,m4.2xlarge,m4.4xlarge,m4.10xlarge,m5.large,m5.xlarge,m5.2xlarge,m5.4xlarge,m5.12xlarge,m5.24xlarge,c4.large,c4.xlarge,c4.2xlarge,c4.4xlarge,c4.8xlarge,c5.large,c5.xlarge,c5.2xlarge,c5.4xlarge,c5.9xlarge,c5.18xlarge,i3.large,i3.xlarge,i3.2xlarge,i3.4xlarge,i3.8xlarge,i3.16xlarge,r3.xlarge,r3.2xlarge,r3.4xlarge,r3.8xlarge,r4.large,r4.xlarge,r4.2xlarge,r4.4xlarge,r4.8xlarge,r4.16xlarge,x1.16xlarge,x1.32xlarge,p2.xlarge,p2.8xlarge,p2.16xlarge,p3.2xlarge,p3.8xlarge,p3.16xlarge
	// +optional
	InstanceType string `json:"instanceType,omitempty"`
	// Default Value is 3
	// +kubebuilder:validation:Maximum=10
	// +kubebuilder:validation:Minimum=1
	// +optional
	MaxInstanceCount int `json:"maxInstanceCount,omitempty"`
}

// NodeGroupStatus defines the observed state of NodeGroup
type NodeGroupStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Status string `json:"status"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NodeGroup is the Schema for the nodegroups API
// +k8s:openapi-gen=true
type NodeGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeGroupSpec   `json:"spec,omitempty"`
	Status NodeGroupStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NodeGroupList contains a list of NodeGroup
type NodeGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeGroup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NodeGroup{}, &NodeGroupList{})
}

func (ng *NodeGroup) GetVersion() string {
	if isSupportedVersion(ng.Spec.Version) {
		return *ng.Spec.Version
	}
	return DefaultVersion
}
