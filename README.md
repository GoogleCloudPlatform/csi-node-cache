# Kubernetes Node Cache

This repository deploys a CSI ephemeral driver for a shared node volume. All
pods that use it see the same volume, and this volume outlives any pod. It can
be thought of a a replacement for hostPath volumes, although it also gives more
options for the storage medium.

It currently supports ram volumes, local SSD, and block storage (persistent disk
and hyperdisk). The local SSD and block storage support is specific to GKE
(Google's managed kubernetes service on GCP).

## Note

This is not an officially supported Google product. This project is not
eligible for the [Google Open Source Software Vulnerability Rewards
Program](https://bughunters.google.com/open-source-security).

## Set Up

### Artifact Registry
* Create repositories. You choose a host; the default is us-docker.pkg.dev. Then
  you create a repository within the host. Images go inside this repository. The
  final image path looks like HOST/PROJECT/REPO/IMAGE:TAG. See [GCP AR
  documentation](https://cloud.google.com/artifact-registry/docs/docker/store-docker-container-images#create).
* Your local docker needs access to be able to push, the is done with, eg, `gcloud auth configure-docker us-docker.pkg.dev`.
* GKE by default can pull from any artifact registry repo in its own project. If
  you're pulling cross-project, see the [access control
  documentation](https://cloud.google.com/artifact-registry/docs/access-control#gke).

### Workload Identity
If you use the PD cache, you must set up workload identity. See [PD
Caches](#pd-caches) for details.

## Deployment

We do not host pre-built images. You will need to build your own images from
this repository and push them to a convenient image repository reachable from
your cluster.

Run `make images PROJECT=${YOUR_PROJECT_ID} REPO=${YOUR_AR_REPO}`. This will
build and push images to `gcr.io/${YOUR_PROJECT_ID}`. The image names and tags
can be customized with `TAG`, `DRIVER_IMAGE_NAME` and
`CONTROLLER_IMAGE_NAME`. The artifact registry host can be updated with
REPO_HOST if you aren't in us-docker.pkg.dev. If you don't give the REPO
parameter, images will be pushed to the legacy gcr.io/${YOUR_PROJECT} repository
instead.

The image locations are plumbed to the yaml via kustomize. To deploy, run the following.

```
kubectl apply -k deploy/
```

The difference between `-k`- and `-f` is significant, be careful --- `kubectl
apply -f` will fail and leave your cluster in a broken state. If you
accidentally do this, delete the `node-cache` namespace and retry.

Note that the deployment ensures the controller runs on a node with workload
identity, in case  the PD cache is used. If you will not be using the PD cache
and you don't want to set up workload identity for your cluster, you can remove
the controller node selector.

## Use

Appropriately label nodes where you want a cache to be used.

The label key is `node-cache.gke.io`. Values may be:

* **tmpfs**. This creates a ramdisk that persists across pod restarts. The label
  `node-cache-size.gke.io` must also be on the node, which sets the size of
  this disk in bytes. It uses k8s resource units, for example,
  `node-cache-size.gke.io=50Gi`.

  The memory accounting for this ramdisk is a little confusing -- it appears
  that usage can be accounted to the writing container, rather than the CSI
  driver. Something to figure out before this is used in production.

* **lssd**. This will raid local SSD into a cache that persists across pod
  restarts. The node should be created with `--local-nvme-ssd-block` flag. All
  local ssd cards will be used for the cache.

* **pd**. A persistent disk will be created for the
  cache. `node-cache-size.gke.io` must be set (it uses standard k8s parsing, eg
  50Gi). See **PD Caches** below for more details.

See `examples/example-pod.yaml` for a simple example. The pod should have a node
selector for the nodes that have been set up with the desired kind of node
cache.

If no such label is present on a node, it cannot be used with a cache
volume. Pods with a cache volume scheduled to such a node will be stuck in
pending.

## PD Caches

Caches based on persistent disk are created with the `node-cache.gke.io` storage
class, which may be customized as part of this deployment. Keep the volume
binding mode as immediate, however, or volume creation will break.

The controller will create a PVC in order to provision the volume for a
node. This PVC will be marked with a `node-cache.gke.io` finalizer and
immediately deleted so that it cannot be used with a pod. The finalizer will
keep the volume from being reclaimed. The controller will then manually attach
the volume to the node. There is no detach operation. The controller will delete
such PVCs when there is no corresponding node (by removing the finalizer).

The PVC is created for any node labeled with `node-cache.gke.io=pd`, whether or
not there is a pod using the cache on that node.

The node must also hvae the `node-cache-size.gke.io` label set in order to
create a volume. Pods will be stuck pending until this is done.

The controller service account must be linked to a GCP service account through
workload identity. This SA needs a role with compute.instances.attachDisk IAM
permissions in order to attach the disk.

### Workload Identity Setup

https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity

* Create or update the cluster with
  `--workload-pool=${PROJECT_ID:?}.svc.id.goog` and node pools with
  `--workload-metadata=${GKE_METADATA:?}`. The latter can be used to tell if a node
  pool is already opted in to WI.

* Create a custom role.
  - Create a yaml file containinging the following
    ```
    title: node-cache-role
    description: PD manipulation for the csi node cache
    stage: GA
    includedPermissions:
    - compute.disks.get
    - compute.disks.list
    - compute.disks.use
    - compute.instances.attachDisk
    - compute.instances.get
    - compute.instances.use
    ```
  - `gcloud iam roles create nodeCache --project=${PROJECT_ID:?} --file="${YAML_FILE_PATH}"`
* Add roles to a k8s SA:
  ```
  gcloud projects add-iam-policy-binding ${PROJECT_ID:?} \
    --role=projects/${PROJECT_ID:?}/roles/nodeCache \
    --member=principal://iam.googleapis.com/projects/${PROJECT_NUMBER:?}/locations/global/workloadIdentityPools/${PROJECT_ID:?}.svc.id.goog/subject/ns/node-cache/sa/node-cache-controller \
    --condition=None
  gcloud iam service-accounts add-iam-policy-binding \
    PROJECT_NUMBER-compute@developer.gserviceaccount.com \
    --member
  principal://iam.googleapis.com/projects/${PROJECT_NUMBER:?}/locations/global/workloadIdentityPools/${PROJECT_ID:?}.svc.id.goog/subject/ns/node-cache/sa/node-cache-controller \
  --role roles/iam.serviceAccountUser --condition=None
```

Workload identity can be difficult to debug. If pods trying to use a PD cache
are hanging, look for authentication errors in the controller logs (`kubectl
logs -n node-cache -l app=controller`). Double-check the instructions
above. Deploy `examples/wi-test.yaml`. This creates a pod `wi-test` in the
`node-cache` namespace running under the same service account as the
controller. After printing out what the GCP application default credential
service account looks like, it sleeps so that you can exec into it for further
debugging.
