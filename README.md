This is a k8s controller for running and managing SR Linux nodes launched from [openconfig/kne](https://github.com/openconfig/kne) topology.

## Install
To install the latest version of this controller on a cluster referenced in `~/.kube/config` issue the following command:
```bash
# latest version
kubectl apply -k https://github.com/srl-labs/srl-controller/config/default

# specific version
kubectl apply -k https://github.com/srl-labs/srl-controller/config/default?ref=v0.3.1
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
If this repo is cloned, the controller can be installed with make:
```
make deploy IMG=ghcr.io/srl-labs/srl-controller:0.3.1
```

Make sure to check which controller versions are [available](https://github.com/srl-labs/srl-controller/pkgs/container/srl-controller/versions)

## Uninstall
To uninstall the controller from the cluster:
```
kubectl delete -k https://github.com/srl-labs/srl-controller/config/default
```

## Testing with `kind`
To run this controller in a test cluster deployed with [`kind`](https://kind.sigs.k8s.io/) follow the steps outlined below.

1. Install `kind`
2. Clone and enter into [openconfig/kne](https://github.com/openconfig/kne) repo.
3. Build the kne cli with  
   `cd kne_cli && go build -o kne && chmod +x ./kne && mv ./kne /usr/local/bin`
4. deploy kind cluster and the necessary CNI with `kne deploy deploy/kne/kind.yaml` where the path to `kind.yaml` is a relative path from the root of the kne repo.

This will install the `kind` cluster named `kne` with [`meshnet-cni`](https://github.com/networkop/meshnet-cni) and [`metallb`](https://metallb.universe.tf/).

`kind` clusters use `ptp` cni plugin which installs a route in the pods default netns to implement the routing behavior. This will not work with SR Linux pods, as their management network is not using the default namespace as explained [here](https://github.com/kubernetes-sigs/kind/issues/2444).

To workaround the mismatch between `ptp` plugin expectations and SR Linux netns implementation the kne users need to make changes to the CNI chaining and swap `ptp` plugin with `bridge` plugin. To do this execute the [following script](https://gist.github.com/hellt/806e6cc8d6ae49e2958f11b4a1fc3091) on a kind cluster control plane node:

```
docker exec kne-control-plane bash -c "curl https://gist.githubusercontent.com/hellt/806e6cc8d6ae49e2958f11b4a1fc3091/raw/8f45ad34f60b6128af78b4766aa4cae7b54bf881/bridge.sh | /bin/bash"
```

Before proceeding a quick test may be performed to verify that SR Linux pods can communicate over the newly installed bridge plugin:

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

If all works as expected a [demo topology with three SR Linux nodes](https://github.com/openconfig/kne/blob/main/examples/3node-srl.pb.txt) may be deployed as follows:

```bash
kne create ~/kne/examples/3node-srl.pb.txt
```

This will deploy the SR Linux nodes and will create k8s services as per the topology configuration. The services will be exposed via MetalLB and can be queried as:

```
❯ kubectl -n 3node-srlinux get svc
NAME         TYPE           CLUSTER-IP      EXTERNAL-IP   PORT(S)                                      AGE
service-r1   LoadBalancer   10.96.151.84    172.19.0.50   57400:30006/TCP,443:30004/TCP,22:30005/TCP   6m10s
service-r2   LoadBalancer   10.96.34.36     172.19.0.51   443:30010/TCP,22:30011/TCP,57400:30009/TCP   6m9s
service-r3   LoadBalancer   10.96.159.220   172.19.0.52   443:30015/TCP,22:30016/TCP,57400:30014/TCP   6m9s
```

To connect with SSH to r1 node, use `ssh admin@172.19.0.50` command.

## Controller operations
The controller is designed to manage the `Srlinux` custom resource defined with [the following CRD](https://doc.crds.dev/github.com/srl-labs/srl-controller).

The request to create/delete a resource of kind `Srlinux` is typically coming from `openconfig/kne` topology.

### Creation
When a request to create an `Srlinux` resource named `r1` in namespace `ns` comes in, controller's reconcile loop does the following:

1. Checks if the pods exists within a namespace `ns` with a name `r1` 
2. If the pod hasn't been found, then controller first ensures that the necessary config maps exist in namespace `ns` and creates them otherwise.
3. When config maps are sorted out, controller schedules a pod with name `r1` and requeue the request
4. In a requeue run, the pod is now found and controller updates the status of `Srlinux` resource with the image name that was used in the pod spec.

### Deletion
When a deletion happens on `Srlinux` resource, the reconcile loop does nothing.

### API access
This repo contains a clientset for API access to the `Srlinux` custom resource. Check [kne repo](https://github.com/openconfig/kne/blob/fc195a73035bcbf344791979ca3e067be47a249c/topo/node/srl/srl.go#L46) to see how this can be done.
