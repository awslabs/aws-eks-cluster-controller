package controller

import (
	"github.com/awslabs/aws-eks-cluster-controller/pkg/controller/controlplane"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, controlplane.Add)
}
