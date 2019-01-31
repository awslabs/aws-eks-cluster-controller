package eks

import (
	"context"
	"fmt"

	componentsv1alpha1 "github.com/awslabs/aws-eks-cluster-controller/pkg/apis/components/v1alpha1"
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/runtime"
)

var componentsToDelete = []runtime.Object{
	&componentsv1alpha1.ConfigMapList{},
	&componentsv1alpha1.DeploymentList{},
	&componentsv1alpha1.IngressList{},
	&componentsv1alpha1.SecretList{},
	&componentsv1alpha1.ServiceList{},
}

func deleteComponents(ownerName, ownerNamespace string, c client.Client, logger *zap.Logger) (int, error) {

	delete := []runtime.Object{}
	for _, componentList := range componentsToDelete {
		list := componentList.DeepCopyObject()
		err := c.List(context.TODO(), client.MatchingLabels(map[string]string{
			"eks.owner.name":      ownerName,
			"eks.owner.namespace": ownerNamespace,
			"eks.needsdeleting":   "true",
		}), list)
		if err != nil {
			return 0, nil
		}
		switch l := list.(type) {
		case *componentsv1alpha1.ConfigMapList:
			for _, obj := range l.Items {
				item := obj
				delete = append(delete, &item)
			}
		case *componentsv1alpha1.DeploymentList:
			for _, obj := range l.Items {
				item := obj
				delete = append(delete, &item)
			}
		case *componentsv1alpha1.IngressList:
			for _, obj := range l.Items {
				item := obj
				delete = append(delete, &item)
			}
		case *componentsv1alpha1.SecretList:
			for _, obj := range l.Items {
				item := obj
				delete = append(delete, &item)
			}
		case *componentsv1alpha1.ServiceList:
			for _, obj := range l.Items {
				item := obj
				delete = append(delete, &item)
			}
		case *componentsv1alpha1.ClusterRoleList:
			for _, obj := range l.Items {
				item := obj
				delete = append(delete, &item)
			}
		case *componentsv1alpha1.ClusterRoleBindingList:
			for _, obj := range l.Items {
				item := obj
				delete = append(delete, &item)
			}
		case *componentsv1alpha1.ServiceAccountList:
			for _, obj := range l.Items {
				item := obj
				delete = append(delete, &item)
			}
		default:
			logger.Error("Got object type we didn't understand", zap.Any("object", l))
			return 0, fmt.Errorf("unknown type error")
		}

	}

	errs := []error{}
	for _, obj := range delete {
		err := c.Delete(context.TODO(), obj)
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return len(delete), fmt.Errorf("error deleting objects %v", errs)
	}

	return len(delete), nil
}
