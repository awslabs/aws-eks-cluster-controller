aws-eks-cluster-controller models following resources as Kubernetes [CustomResourceDefinitions(CRDs)](https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definitions/):

### Supported Custom Resources

1. Clusters
   1. EKS Controlplane
   1. EKS Nodegroups
1. Components
1. [Deployment](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.13/#deployment-v1-apps)
1. [Service](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.13/#service-v1-core)
1. [ConfigMap](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.13/#configmap-v1-core)
1. [Ingress](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.13/#ingress-v1beta1-extensions)
1. [Secret](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.13/#secret-v1-core)
1. [Secret](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.13/#secret-v1-core)
1. [ServiceAccount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.13/#serviceaccount-v1-core)
1. [ClusterRole](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.13/#clusterrole-v1-rbac-authorization-k8s-io)
1. [ClusterRoleBinding](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.13/#clusterrolebinding-v1-rbac-authorization-k8s-io)
1. [StatefulSet](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.13/#statefulset-v1-apps)
1. [DaemonSet](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.13/#daemonset-v1-apps)
1. [CustomResourceDefinition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.13/#customresourcedefinition-v1beta1-apiextensions-k8s-io)

### Future Resources

We are tracking the progress of the [Federation V2 Kubernetes SIG](https://github.com/kubernetes-sigs/federation-v2) and plan to migrate to that once it is stable.
