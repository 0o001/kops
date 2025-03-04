# Getting Started with kOps on OpenStack

OpenStack support on kOps is currently **beta**, which means that OpenStack support is in good shape and could be used for production. However, it is not as rigorously tested as the stable cloud providers and there are some features not supported. In particular, kOps tries to support a wide variety of OpenStack setups and not all of them are equally well tested.

## OpenStack requirements

In order to deploy a kops-managed cluster on OpenStack, you need the following OpenStack services:

* Nova (compute)
* Neutron (networking)
* Glance (image)
* Cinder (block storage)

In addition, kOps can make use of the following services:

* Swift (object store)
* Dvelve (dns)
* Octavia (loadbalancer)

The OpenStack version should be Ocata or newer.

## Source your openstack RC

The Cloud Config used by the kubernetes API server and kubelet will be constructed from environment variables in the openstack RC file.

```bash
source openstack.rc
```

We recommend using [Application Credentials](https://docs.openstack.org/keystone/queens/user/application_credentials.html) when authenticating to OpenStack.

**Note** The authentication used locally will be exported to your cluster and used by the kubernetes controller components. You must avoid using personal credentials used for other systems,

## Environment Variables

kOps stores its configuration in a state store. Before creating a cluster, we need to export the path to the state store:

```bash
export KOPS_STATE_STORE=swift://<bucket-name> # where <bucket-name> is the name of the Swift container to use for kops state

```

If your OpenStack does not have Swift you can use any other VFS store, such as S3. See the [state store documentation](../state.md) for alternatives.

## Creating a Cluster

```bash
# to see your etcd storage type
openstack volume type list

# coreos (the default) + calico overlay cluster in Default
kops create cluster \
  --cloud openstack \
  --name my-cluster.k8s.local \
  --state ${KOPS_STATE_STORE} \
  --zones nova \
  --network-cidr 10.0.0.0/24 \
  --image <imagename> \
  --master-count=3 \
  --node-count=1 \
  --node-size <flavorname> \
  --master-size <flavorname> \
  --etcd-storage-type <volumetype> \
  --api-loadbalancer-type public \
  --topology private \
  --bastion \
  --ssh-public-key ~/.ssh/id_rsa.pub \
  --networking calico \
  --os-ext-net <externalnetworkname>

# to update a cluster
kops update cluster my-cluster.k8s.local --state ${KOPS_STATE_STORE} --yes

# to delete a cluster
kops delete cluster my-cluster.k8s.local --yes
```

## Optional flags

* `--os-kubelet-ignore-az=true` Nova and Cinder have different availability zones, more information [Kubernetes docs](https://kubernetes.io/docs/concepts/cluster-administration/cloud-providers/#block-storage)
* `--os-octavia=true` If Octavia Loadbalancer api should be used instead of old lbaas v2 api.
* `--os-dns-servers=8.8.8.8,8.8.4.4` You can define dns servers to be used in your cluster if your openstack setup does not have working dnssetup by default
* `--os-octavia-provider` You can define the Octavia Loadbalancer provider to use. To get the list of providers available in your environment, run `openstack loadbalancer provider list`. Default: octavia.

## Compute and volume zone names does not match

Some of the openstack users do not have compute zones named exactly the same than volume zones. Good example is that there are several compute zones for instance `zone-1`, `zone-2` and `zone-3`. Then there is only one volumezone which is usually called `nova`. By default this is problem in kOps, because kOps assumes that if you are deploying things to `zone-1` there should be compute and volume zone called `zone-1`.

However, you can still get kOps working in your openstack by doing following:

Create cluster using your compute zones:

```bash
kops create cluster \
  --zones zone-1,zone-2,zone-3 \
  ...
```

After you have initialized the configuration you need to edit configuration:

```bash
kops edit cluster my-cluster.k8s.local
```

Edit `ignore-volume-az` to `true` and `override-volume-az` according to your cinder az name.

Example (volume zone is called `nova`):

```yaml
spec:
  cloudConfig:
    openstack:
      blockStorage:
        ignore-volume-az: true
        override-volume-az: nova
```

Finally execute update cluster:

```bash
kops update cluster my-cluster.k8s.local --state ${KOPS_STATE_STORE} --yes
```

kOps should create instances to all three zones, but provision volumes from the same zone.

## Using CCM created Loadbalancers

With the default configuration, the loadbalancers created using the [cloud-provider-openstack](https://github.com/kubernetes/cloud-provider-openstack) cloud controller provider do not have access to the exposed NodePorts.

A fix is to add the clouster network to the authorized nodeIds.

First, you have to edit the cluster:

```bash
kops edit cluster <cluster>
```

Add the following to the clusterspec:
```yaml
spec:
  nodePortAccess:
    - <Your network CIDR>
```

Finally, update the cluster:
```bash
kops update cluster --name <cluster> --yes
```

## Using OpenStack without lbaas

Some OpenStack installations does not include installation of lbaas component. To launch a cluster without a loadbalancer, run:

```bash
kops create cluster \
  --cloud openstack \
  ... (like usually)
  --api-loadbalancer-type=""
```

In clusters without loadbalancer, the address of a single random master will be added to your kube config.

## Using existing OpenStack network

You can have kOps reuse existing network components instead of provisioning one per cluster. As OpenStack support is still beta, we recommend you take extra care when deleting clusters and ensure that kOps do not try to remove any resources not belonging to the cluster.

### Let kOps provision new subnets within an existing network

Use an existing network by using `--network <network id>`.

If you are provisioning the cluster from a spec file, add the network ID as follows:

```yaml
spec:
  networkID: <network id>
```

### Use existing subnets

Instead of kOps creating new subnets for the cluster, you can reuse an existing subnet.

When you create a new cluster, you can specify subnets using the `--subnets` and `--utility-subnets` flags.

Example:

```bash
kops create cluster \
  --cloud openstack \
  --name sharedsub2.k8s.local \
  --state ${KOPS_STATE_STORE} \
  --zones zone-1 \
  --network-cidr 10.1.0.0/16 \
  --image debian-10-160819-devops \
  --master-count=3 \
  --node-count=2 \
  --node-size m1.small \
  --master-size m1.small \
  --etcd-storage-type default \
  --topology private \
  --bastion \
  --networking calico \
  --api-loadbalancer-type public \
  --os-kubelet-ignore-az=true \
  --os-ext-net ext-net \
  --subnets c7d20c0f-df3a-4e5b-842f-f633c182961f \
  --utility-subnets 90871d21-b546-4c4a-a7c9-2337ddf5375f \
  --os-octavia=true --yes
```

## Using with self-signed certificates in OpenStack

kOps can be configured to use insecure mode towards OpenStack. However, this is not recommended as OpenStack cloudprovider in kubernetes does not support it.
If you use insecure flag in kOps it might be that the cluster does not work correctly.

```yaml
spec:
  cloudConfig:
    openstack:
      insecureSkipVerify: true
```

## Advanced Instance Group config

### Adding allowed address pairs to ports

It is possible to make kOps provision and update the ports of the servers with allowed address pairs, which can be beneficial when needing to use for example VRRP for a custom loadbalancing solution.

To make use of this annotate an Instance Group configuration like so:

```yaml
kind: InstanceGroup
metadata:
  annotations:
    openstack.kops.io/allowedAddressPair/N: <IPAddress>(,<MACAddress>)
```

Where `N` can be an arbitrary identifier and the MACAddress is optional, for example:

```yaml
kind: InstanceGroup
metadata:
  annotations:
    openstack.kops.io/allowedAddressPair/0: 192.168.0.0/16
    openstack.kops.io/allowedAddressPair/1: 10.123.0.10,12:34:56:78:90:AB
```

For more information about allowed address pairs refer to the [OpenStack Network API documentation](https://docs.openstack.org/api-ref/network/v2/#allowed-address-pairs).

### Using boot from volume

By default kOps provisions servers with "boot from image".

To use "boot from volume" for servers specify the following annotations in the respective Instance Group manifests:

```yaml
kind: InstanceGroup
metadata:
  annotations:
    openstack.kops.io/osVolumeBoot: true
    openstack.kops.io/osVolumeSize: 10
```

Setting the size of the volume with `osVolumeSize` is optional and if not specified kOps will use the value of the image's minimum amount of disk space required to boot it. The value of it needs to be an integer and is the mount of GBs the root volume will use.

Please note that when enabling "boot from volume" the servers will use the default volume type and the volumes will be deleted when the servers are terminated.

### Using a custom server group policy

By default kOps provisions the server groups in OpenStack with `anti-affinity`.

To override this add the following annotation to the Instance Group configuration where kOps should provision a server group with another policy:

```yaml
kind: InstanceGroup
metadata:
  annotations:
    openstack.kops.io/serverGroupAffinity: soft-anti-affinity
```

Please refer to the [OpenStack Compute API documentation](https://docs.openstack.org/api-ref/compute/?expanded=create-server-group-detail#create-server-group) for supported policies.

### Using a custom server group name

By default kOps provisions the server groups in OpenStack with `anti-affinity`.
However, if you have only one AZ and you want to run multiple control-planes you might want to have anti-affinity between control planes.

To override this add the following annotation to all control-plane Instance Groups configuration where kOps should provision a server group with another policy:

```yaml
kind: InstanceGroup
metadata:
  annotations:
    openstack.kops.io/serverGroupName: control-plane
```

## Next steps

Now that you have a working kOps cluster, read through the [recommendations for production setups guide](production.md) to learn more about how to configure kOps for production workloads.
