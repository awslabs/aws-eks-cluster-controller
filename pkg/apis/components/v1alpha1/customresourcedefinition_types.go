// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package v1alpha1

import (
	apiextv1beta "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CustomResourceDefinitionSpec defines the desired state of CustomResourceDefinition
type CustomResourceDefinitionSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	apiextv1beta.CustomResourceDefinitionSpec `json:",inline"`

	Cluster string `json:"cluster"`
	Name    string `json:"name"`
}

// CustomResourceDefinitionStatus defines the observed state of CustomResourceDefinition
type CustomResourceDefinitionStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Status string `json:"status,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CustomResourceDefinition is the Schema for the customresourcedefinitions API
// +k8s:openapi-gen=true
type CustomResourceDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CustomResourceDefinitionSpec   `json:"spec,omitempty"`
	Status CustomResourceDefinitionStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CustomResourceDefinitionList contains a list of CustomResourceDefinition
type CustomResourceDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CustomResourceDefinition `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CustomResourceDefinition{}, &CustomResourceDefinitionList{})
}
