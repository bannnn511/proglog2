# proglog

## What is this project?

    - A distributed system for storing and replicating logs.

## What is the purpose of this project?

    - To learn about distributed systems.

## What do you learn from this project?

    - Build a log package
    - Serve request with GRPC
    - Observer the system with tracing
    - Server to server service discovery using Hashicorp Serf
    - Server replication using service discovery
    - Cordinate service using Raft for consensus algorithm
    - Loadbalance using GRPC resolver
    - Start server locally using K8s and Helm

## How to run this project?

    1. Build project image:
        - make build-image (provide your own image name)
    2. Update values for helm in `deploy/proglog/values.yaml`
        1. Update `image.repository` with your image name
        2. Update `image.tag` with your image tag
    3. (Optional)For persistence, you need to create a storage class and a persistent volume
        1. create custom local cluster for kind:
            - kind create cluster --config deploy/prolog/kind-config.yaml
        1. Create storage class:
            - kubectl apply -f deploy/storage-class.yaml
        2. Create persistent volume for each replicas using template (3 replicas - 3 pvs):
            - kubectl apply -f deploy/persistent-volume-claim-template.yaml
        3. Update statefulset volume claim template to use storage class
    4. Install helm chart:
        1. cd deploy/proglog
        2. helm install proglog deploy/proglog

## Deploy to GKE

    1. Create service account:
        - `gcloud iam service-accounts create $SA_NAME`
    2. Retrieve the email address of the service account you just created:
      - `gcloud iam service-accounts list`
    3. Get project id:
      - `gcloud projects list | grep 'distributed' |tail -n 1 | cut -d' ' -f1`
    4. Add roles to the service account. Note: Apply more restrictive roles to suit your requirements.
      - `gcloud projects add-iam-policy-binding $GKE_PROJECT \
        --member=serviceAccount:$SA_EMAIL \
        --role=roles/container.admin`
      - `gcloud projects add-iam-policy-binding $GKE_PROJECT \
        --member=serviceAccount:$SA_EMAIL \
        --role=roles/storage.admin`
      - `gcloud projects add-iam-policy-binding $GKE_PROJECT \
        --member=serviceAccount:$SA_EMAIL \
        --role=roles/container.clusterViewer`