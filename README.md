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

## Testing with `kind`
To run this controller in a test cluster deployed with [`kind`](https://kind.sigs.k8s.io/) follow the steps outlined below.

1. Install `kind`
2. Clone and enter into [google/kne](https://github.com/google/kne) repo.
3. Build the kne cli with  
   `cd kne_cli && go build -o kne && chmod +x ./kne && mv ./kne /usr/local/bin`
4. deploy kind cluster and the necessary CNI with `kne deploy deploy/kne/kind.yaml` where the path to `kind.yaml` is a relative path inside the kne repo.

This will install the `kind` cluster named `kne` with `meshnet-cni` and `metallb`.

`kind` clusters use `ptp` cni plugins which install the route in the pods default netns to implement the routing behavior. This will not work with SR Linux pods, as their management network is not using the default namespace as explained [here](https://github.com/kubernetes-sigs/kind/issues/2444).

To overcome the mismatch between `ptp` expectations and SR Linux netns slicing the users need to make changes to the CNI chaining used by `kind` and swap ptp plugin with bridge plugin. To do this, you need to execute the [following script](https://gist.github.com/hellt/806e6cc8d6ae49e2958f11b4a1fc3091) on kind cluster:

```
docker exec kne-control-plane bash -c "curl https://gist.githubusercontent.com/hellt/806e6cc8d6ae49e2958f11b4a1fc3091/raw/5b4cab0a8f00d23e55dec924233dd4a1acaebc88/bridge.sh | /bin/bash"
```

The script must be executed without errors.

Before proceeding a quick test may be run to verify that SR Linux pods can communicate with the bridge plugin:

```bash
# apply this manifest https://gist.github.com/hellt/43cfade6178be32ea7dfa5cb64715822
# which has two srlinux pods and two linux pods
kubectl apply -f https://gist.githubusercontent.com/hellt/43cfade6178be32ea7dfa5cb64715822/raw/847a10a57dca996432be7c4a9743c0e0c5b75814/srl.yml

# once the pods are deployed and running, verify that srlinux pods can reach each other with ssh
# check what IP the srl2 has
kubectl exec -it srl1 -- ip netns exec srbase-mgmt ssh admin@10.244.0.7
admin@10.244.0.7's password: 
Last login: Fri Sep 24 13:58:05 2021 from 10.244.0.6
Using configuration file(s): ['/etc/opt/srlinux/srlinux.rc']
Welcome to the srlinux CLI.
Type 'help' (and press <ENTER>) if you need any help using this.
--{ [FACTORY] running }--[  ]--   
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