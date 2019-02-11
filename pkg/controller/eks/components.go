package eks

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"k8s.io/apimachinery/pkg/runtime/schema"

	crdv1b1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"

	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func deleteComponents(ownerName, ownerNamespace string, c client.Client, logger *zap.Logger) (count int, err error) {

	listOptions := client.MatchingLabels(map[string]string{
		"eks.owner.name":      ownerName,
		"eks.owner.namespace": ownerNamespace,
		"eks.needsdeleting":   "true",
	})

	crds := &crdv1b1.CustomResourceDefinitionList{}
	err = c.List(context.TODO(), &client.ListOptions{}, crds)
	if err != nil {
		logger.Error("error getting crds", zap.Error(err))
		return count, err
	}
	gvks := []schema.GroupVersionKind{}
	for _, crd := range crds.Items {

		version := crd.Spec.Version
		if len(crd.Spec.Versions) > 0 {
			version = crd.Spec.Versions[0].Name
		}
		if crd.Spec.Group == "components.eks.amazonaws.com" {
			gvks = append(gvks, schema.GroupVersionKind{
				Group:   crd.Spec.Group,
				Version: version,
				Kind:    crd.Spec.Names.ListKind,
			})
		}
	}

	for _, gvk := range gvks {
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(gvk)

		c.List(context.TODO(), listOptions, list)

		if len(list.Items) > 0 {
			for _, item := range list.Items {
				count++
				err = c.Delete(context.TODO(), &item)
				if err != nil {
					logger.Error("error deleting resource", zap.String("resource", gvk.String()), zap.Error(err))
					return count, err
				}
			}
		}
	}
	return count, nil
}
