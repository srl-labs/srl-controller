# Using license files

To remove the packets-per-second limit of a public container image or to launch chassis-based variants of SR Linux (ixr-6e/10e) KNE users should provide a valid license file to the `srl-controller`.

License file provisioning is handled by the `srl-controller`. Users should create a k8s secret with license text blobs stored as keys for a controller to pick it up and use with SR Linux pods. In the next sections, we provide examples for the complete workflow of license provisioning.

## Creating a secret with licenses

SR Linux license file can contain a single license blob for a certain release or pack multiple license blobs for several versions. SR Linux NOS can conveniently find a matching license in a file that contains several licenses automatically.

A license file with multiple licenses can look similar to that:

```text
#
# srl
#
00000000-0000-0000-0000-000000000000 aACUAsYXC0NTA1NERLABiSBETTERtHAnKNEAAAAA  # srl_rel_22_03_*
00000000-0000-0000-0000-000000000000 aACUAsYXC0NTA1NERLABiSBETTERtHAnKNEAAAAA  # srl_rel_22_06_*
00000000-0000-0000-0000-000000000000 aACUAsYXC0NTA1NERLABiSBETTERtHAnKNEAAAAA  # srl_rel_22_11_*
```

A file like that contains licenses for SR Linux releases `22.3.*`, `22.6.*`, `22.11.*`, but, for instance, not for `21.11.*`.

Because a single license file can contain multiple license blob, users can maintain a single file and append license blobs to it as the new releases come out.

For the sake of an argument, let's assume that a license file that contains license blobs is named `licenses.key` and exists in a current working directory. With a license file available, users should create a Secret in the `srlinux-controller` namespace with a license file blob contained under the `all.key` key.

```bash
kubectl create namespace srlinux-controller; \
kubectl create -n srlinux-controller \
    secret generic srlinux-licenses --from-file=all.key=licenses.key \
    --dry-run=client --save-config -o yaml | \
    kubectl apply -f -
```

> **Note**  
> The above snippet ensures that `srlinux-controller` namespace exists, and then creates a Secret object from `licenses.key` file and puts its content under `all.key` key.

Now you should have a Secret object in `srlinux-controller` namespace that contains SR Linux licenses.

## License mount

Once a Secret with license information is created, SR Linux pods will have a new volume mounted by the controller with the contents of the original license file by the path `/opt/srlinux/etc/license.key`.

SR Linux NOS then will read this file at startup and will use a license if a valid string is found in that file. If no valid license is found the system will boot as if no license file was provided.

## Updating licenses

If you wish to add/remove a license to/from your collection of licenses you simply modify the existing file which you used to create a Secret object from and reinvoke the same command.
