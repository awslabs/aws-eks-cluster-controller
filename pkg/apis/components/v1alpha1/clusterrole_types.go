package v1alpha1

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterRole is the Schema for the serviceaccounts API
// +k8s:openapi-gen=true
type ClusterRole struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterRoleSpec   `json:"spec,omitempty"`
	Status ClusterRoleStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterRoleList contains a list of ClusterRole
type ClusterRoleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterRole `json:"items"`
}

// ClusterRoleSpec defines the desired state of Secret
type ClusterRoleSpec struct {
	rbacv1.ClusterRole `json:",inline"`
	Cluster            string `json:"cluster"`
	Name               string `json:"name"`
}

// ClusterRoleStatus defines the observed state of ClusterRole
type ClusterRoleStatus struct {
	Status string `json:"status,omitempty"`
}

func init() {
	SchemeBuilder.Register(&ClusterRole{}, &ClusterRoleList{})
}
