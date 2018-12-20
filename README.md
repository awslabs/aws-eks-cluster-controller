![CircleCI](https://circleci.com/gh/awslabs/aws-eks-cluster-controller.svg?style=svg&circle-token=5f800668d4109bde7cae271f9faa2500e7e33461)(https://circleci.com/gh/awslabs/aws-eks-cluster-controller)

## AWS EKS Cluster Controller

aws-eks-cluster-controller manages EKS clusters in different AWS accounts. It provides CRUD like functionality and deployment of Kubernetes resources to remote EKS clusters.

This controller is build on [kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) framework. Refer https://book.kubebuilder.io/ for more detailed information about kubebuilder.

aws-eks-cluster-controller models following resources as Kubernetes [CustomResourceDefinitions(CRDs)](https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definitions/):

1. EKS Clusters
   1. EKS Controlplane
   1. EKS Nodegroups
1. Kubernetes Deployments
1. Kubernetes Services
1. Kubernetes ConfigMaps
1. Kubernetes Ingress
1. Kubernetes Secrets

### Concepts
- Parent EKS Cluster: The kubernetes cluster where this controller runs. This is where CRDs are created and controller watches the changes on those CRDs.
- Child EKS Clusters: These are the kubernetes clusters managed by the controller running in parent EKS cluster. We may sometimes refer them as target/remote clusters. These child clusters can be run in separate AWS accounts.

### Installation

#### Prerequisites
Make sure you have following tools installed on your workstation:
1. [aws-cli](https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-install.html)
1. [GO development environment](https://golang.org/doc/install)
1. [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl)
1. [kubebuilder](https://github.com/kubernetes-sigs/kubebuilder)
1. [kustomize](https://github.com/kubernetes-sigs/kustomize)

#### Setup Parent EKS cluster
1. [Configure AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-configure.html), so we can access the AWS account. We will call this parent AWS account.
1. Create the Parent EKS cluster - [eksctl](https://github.com/weaveworks/eksctl) provides a simple way to get up and running, otherwise [go here](https://docs.aws.amazon.com/eks/latest/userguide/getting-started.html) and set up using aws cli.
1. Verify you can access the created parent EKS cluster using kubectl.
    For example,
    ```
    kubectl get nodes
    NAME                              STATUS    ROLES     AGE       VERSION
    ip-192-168-108-86.ec2.internal    Ready     <none>    1d       v1.10.3
    ip-192-168-171-52.ec2.internal    Ready     <none>    1d       v1.10.3
    ip-192-168-254-241.ec2.internal   Ready     <none>    1d       v1.10.3
    ```
   If you have created cluster using `eksctl`, you may use `eksctl utils write-kubeconfig` option to configure kubeconfig file. *OR*
   You can use `aws eks update-kubeconfig --name <cluster_name>` option to configure kubeconfig file.
1. Download the package
    ```
    go get github.com/awslabs/aws-eks-cluster-controller
    cd $GOPATH/src/github.com/awslabs/aws-eks-cluster-controller
    ```
1. First time deploy the CRDs or when you update CRD definition
    ```
    make install
    ```
    *** Follow the steps if you wish to run controller as StatefulSet in your parent EKS cluster, if you are running it from your workstation(for development or debugging) you can ignore them ***
1. Create your role for use with kube2iam
    ```
    EKS_NODE_WORKER_ARN=<EKS_NODE_WORKER_ARN>; \
    aws cloudformation create-stack \
        --stack-name aws-eks-controller-role \
        --capabilities CAPABILITY_IAM CAPABILITY_NAMED_IAM \
        --template-body file://config/setup/aws-eks-cluster-controller-role.yaml \
        --parameters \
            ParameterKey=WorkerArn,ParameterValue=${EKS_NODE_WORKER_ARN}
    export IAMROLEARN=`aws iam get-role --role-name aws-eks-cluster-controller --query 'Role.Arn' --output text`
    ```
1. Setup [kube2iam](https://github.com/jtblin/kube2iam) running in parent cluster
    ```
    kubectl apply -f config/setup/kube2iam.yaml
    ```
1. Create ECR repo and get the repository URI
    ```
    aws ecr create-repository --repository-name aws-eks-cluster-controller
    export REPOSITORY=`aws ecr describe-repositories --repository-name aws-eks-cluster-controller --query 'repositories[0].repositoryUri' --output text`
    ```
1. Build and push docker image to ECR repo
    ```
    IMG=${REPOSITORY}:latest IAMROLEARN=${IAMROLEARN} make docker-build
    aws ecr get-login --no-include-email | bash -
    docker push ${REPOSITORY}:latest
    ```

##### Run controller locally
1. Run the controller
    ```
    make run
    ```
    Generally you will iterate on this step in development or testing out something quickly.

##### Run controller as StatefulSet
1. Deploy the image
    ```
    make deploy
    ```

#### Prepare Child AWS account for cross account access
1. Preferably use another terminal and [Configure AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-configure.html), to access the child AWS account
1. Create the role using cloudformation template, which allows access from parent AWS account
    ```
    aws cloudformation create-stack \
        --stack-name aws-eks-cluster-controller-management \
        --capabilities CAPABILITY_IAM CAPABILITY_NAMED_IAM \
        --template-body file://config/setup/aws-eks-cluster-controller-management-role.yaml \
        --parameters \
        ParameterKey=TrustedEntities,ParameterValue=<Comma delimited list of IAM ARNs, User ARNs, etc>
    ```
    In most of the cases you just need to pass Parent EKS cluster's NodeWorker's IAM role associated to EC2 instance. But if you need to pass multiple ARNs you can pass them as comma delimited list.
    For example,
    ```
    ParameterValue="arn:aws:iam::12345678:role/aws-eks-cluster-controller\,arn:aws:iam::12345678:user/myuser"
    ```
    will allow you cross account access from controller running on EKS node-workers in parent EKS cluster as well as using your AWS IAM user's credentials when you run controller from your workstation(`make run`)

### Create resources on child AWS account
Make sure you can access parent EKS cluster using kubectl.

#### Create EKS cluster in child account
* Check the `config/samples/cluster_v1alpha1_eks.yaml` and make necessary changes
    ```
    kubectl apply -f config/samples/cluster_v1alpha1_eks.yaml
    ```
#### Create Deployment in child EKS cluster
* Make changes to `config/samples/components_v1alpha1_deployment.yaml`, if required.
    ```
    kubectl apply -f config/samples/components_v1alpha1_deployment.yaml
    ```
#### Create Service in child EKS cluster
* Make changes to `config/samples/components_v1alpha1_service.yaml`, if required.
    ```
    kubectl apply -f config/samples/components_v1alpha1_service.yaml
    ```
There are sample files in `config/samples` directory for other resources.

## License

This library is licensed under the Apache 2.0 License. 
