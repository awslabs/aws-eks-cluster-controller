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
#### Other samples
There are sample files in `config/samples` directory for other resources.