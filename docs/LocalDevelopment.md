# Local Development
This is the process where you can start building and testing locally. 

### Prerequisites
The same prerequisites from the [readme](../README.md#Prerequisites)

You will also need a kubernetes cluster running, docker comes with one, or follow the steps from the readme [Setup Parent EKS cluster](../README.md#Setup-Parent-EKS-cluster)

Your local environment will need to be able to assume the role that manages the remote environment.

##### Run controller locally
1. Run the controller
    ```
    make run
    ```
    Generally you will iterate on this step in development or testing out something quickly.