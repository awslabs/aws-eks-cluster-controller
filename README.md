[![CircleCI](https://circleci.com/gh/awslabs/aws-eks-cluster-controller.svg?style=svg&circle-token=5f800668d4109bde7cae271f9faa2500e7e33461)](https://circleci.com/gh/awslabs/aws-eks-cluster-controller)

## AWS EKS Cluster Controller

The aws-eks-cluster-controller manages cross account EKS clusters and [supported Kubernetes resources](docs/CurrentComponents.md).

This controller is built using the [kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) framework. For more information [read their docs](https://book.kubebuilder.io/)

### Concepts

- Parent EKS Cluster: The Kubernetes cluster where this controller runs.
- Child EKS Clusters: These are the Kubernetes clusters managed by the controller running in parent EKS cluster.

### Turn Key Installation

#### Prerequisites

Make sure you have following tools installed on your workstation:

1. [aws-cli](https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-install.html)
1. [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl)
1. [eksctl](https://github.com/weaveworks/eksctl)
1. [jq](https://stedolan.github.io/jq/download/)
1. [aws-iam-authenticator](https://github.com/kubernetes-sigs/aws-iam-authenticator#4-set-up-kubectl-to-use-authentication-tokens-provided-by-aws-iam-authenticator-for-kubernetes)
1. [kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) - [install step](https://book.kubebuilder.io/getting_started/installation_and_setup.html)

-- or on MacOS via brew --

```sh
brew tap weaveworks/tap/eksctl
brew install kustomize kubernetes-cli eksctl awscli jq
go get -u -v github.com/kubernetes-sigs/aws-iam-authenticator/cmd/aws-iam-authenticator
```
And [install kubebuilder](https://book.kubebuilder.io/getting_started/installation_and_setup.html)


_IMPORTANT_ make sure your AWS user/role has sufficient permissions to use `eksctl`.

#### Setup Parent EKS cluster

1. Create the Parent EKS cluster

   ```sh
   eksctl create cluster
   ```

1. Once `eksctl` has finished, verify you can access the cluster.

   ```sh
   kubectl get nodes
   ```

1. For this installation process we use [kube2iam](https://github.com/jtblin/kube2iam) to manage IAM permissions for pods running on the parent cluster.

   ```sh
   kubectl apply -f deploy/kube2iam.yaml
   ```

#### Build and deploy the Controller

1. Clone this project

   ```sh
   mkdir -p some/path
   cd some/path
   git clone git@github.com:awslabs/aws-eks-cluster-controller.git
   ```

1. Create the IAM role that the controller will use

   ```sh
   export NODE_INSTANCE_ROLE_ARNS=`aws iam list-roles | jq -r --arg reg_exp "^eksctl-.*-NodeInstanceRole-.*$" '.Roles | map(select(.RoleName|test($reg_exp))) | map(.Arn) | join(",")'`; \

   aws cloudformation create-stack \
    --stack-name aws-eks-controller-role \
    --capabilities CAPABILITY_IAM CAPABILITY_NAMED_IAM \
    --template-body file://config/setup/aws-eks-cluster-controller-role.yaml \
    --parameters \
      ParameterKey=WorkerArn,ParameterValue="'${NODE_INSTANCE_ROLE_ARNS}'"

   export IAMROLEARN=`aws iam get-role --role-name aws-eks-cluster-controller | jq -r .Role.Arn`
   ```

1. Create repository and build/push image

   ```sh
   # Create ECR Repository
   aws ecr create-repository --repository-name aws-eks-cluster-controller
   export REPOSITORY=`aws ecr describe-repositories --repository-name aws-eks-cluster-controller | jq -r '.repositories[0].repositoryUri'`

   # Build/tag the docker image
   IMG=${REPOSITORY}:latest IAMROLEARN=${IAMROLEARN} make docker-build
   
   # Push the docker image
   aws ecr get-login --no-include-email | bash -
   docker push ${REPOSITORY}:latest
   ```

1. Install required Kubernetes CustomResourceDefinitions (CRDs) and deploy controller

   ```sh
   make deploy
   ```

## License

This library is licensed under the Apache 2.0 License. 
