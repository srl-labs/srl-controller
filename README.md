This is a k8s controller for creating SR Linux nodes launched from [google/kne](https://github.com/google/kne) topology.

## Install
To install the latest version of this controller on a cluster referenced in `~/.kube/config` issue the following command:
```
kubectl apply -k https://github.com/srl-labs/kne-controller.git/config/default
```

The resources of this controller will be scoped under `srlinux-controller` namespace.
```
❯ kubectl get all -n srlinux-controller

NAME                                                        READY   STATUS    RESTARTS   AGE
pod/srlinux-controller-controller-manager-c7495dcc7-rbh7m   2/2     Running   0          6m5s

NAME                                                            TYPE        CLUSTER-IP    EXTERNAL-IP   PORT(S)    AGE
service/srlinux-controller-controller-manager-metrics-service   ClusterIP   10.96.34.86   <none>        8443/TCP   16m

NAME                                                    READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/srlinux-controller-controller-manager   1/1     1            1           16m

NAME                                                              DESIRED   CURRENT   READY   AGE
replicaset.apps/srlinux-controller-controller-manager-c7495dcc7   1         1         1       16m
```

### Installing from a repo
If this repo is cloned, the controller can be installed via make target:
```
make deploy IMG=ghcr.io/srl-labs/srl-kne-controller:latest
```

## Uninstall
To uninstall the controller from the cluster:
```
kubectl delete -k https://github.com/srl-labs/kne-controller.git/config/default
```

## Controller operations
The controller is designed to manage the `Srlinux` custom resource defined with [the following CRD](https://github.com/srl-labs/kne-controller/blob/main/config/crd/bases/kne.srlinux.dev_srlinuxes.yaml).

The request to create/delete a resource of kind `Srlinux` is typically coming from `google/kne` topology.

### Creation
When a request to create an `Srlinux` resource named `r1` in namespace `ns` comes in, controller's reconcile loop does the following:

1. Checks if the pods exists within a namespace `ns` with a name `r1` 
2. If the pod hasn't been found, then controller first ensures that the necessary config maps exist in namespace `ns` and creates them otherwise.
3. When config maps are sorted out, controller schedules a pod with name `r1` and requeue the request
4. In a requeue run, the pod is now found and controller updates the status of `Srlinux` resource with the image name that was used in the pod spec.

### Deletion
When a deletion happens on `Srlinux` resource, the reconcile loop does nothing.

### API access
This repo contains a [clientset](https://github.com/srl-labs/kne-controller/blob/645f4c69e888a7aa5f5e87e71e8dde9ec9408620/api/clientset/v1alpha1/srlinux.go) for API access to the `Srlinux` custom resource. Check [kne repo](https://github.com/google/kne/blob/fc195a73035bcbf344791979ca3e067be47a249c/topo/node/srl/srl.go#L46) to see how this can be done.

## Known limitations and the state of development
As of current version `v0.0.0-alpha5` the controller doesn't take into account the node configuration data that might be provided in the kne Topology definition for Srlinux nodes. Instead, the following defaults are used:

* pod image: ghcr.io/nokia/srlinux
* command: `/tini --`
* command args: `fixuid -q /entrypoint.sh sudo bash -c "bash /tmp/topomac/topomac.sh && touch /.dockerenv && /opt/srlinux/bin/sr_linux"`
* sleep time before booting the pod: 0s