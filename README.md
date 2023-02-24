# PaperLB

![Alt text](logo-color.png?raw=true "PaperLB Logo")

A kubernetes network load balancer operator implementation.


**THIS SOFTWARE IS WORK IN PROGRESS / ALPHA RELEASE AND IS NOT MEANT FOR USAGE IN PRODUCTION SYSTEMS**

## What is PaperLB ?
Introduction blog article: [PaperLB. A Kubernetes Network Load Balancer Implementation](https://didil.medium.com/paperlb-fc4c28a82acb) 

You might have noticed that vanilla Kubernetes does not come with a Load Balancer implementation. If you create a LoadBalancer Service in a self-hosted cluster setup, its status will remain "pending" and it won't show an external IP you can use to access the service. It should look something like this:

````bash
$ kubectl get services
NAME                       TYPE           CLUSTER-IP     EXTERNAL-IP   PORT(S)          AGE
k8s-pod-info-api-service   LoadBalancer   10.43.12.233   <pending>     5000:31767/TCP   6s
`````

On the other hand, when you create a LoadBalancer Service in a managed kubernetes cluster such as GCP GKE or AWS EKS, the service will receive an External IP from a Load Balancer assigned by the cloud provider.

The idea behind PaperLB is to allow "LoadBalancer" type services to work with external network load balancers in any environment. PaperLB allows you to use an external L4 Load Balancer of your choice (an nginx server for example) in front of your Kubernetes cluster services. It should work on your development clusters running locally as well as cloud virtual machines or bare metal.

![Alt text](paperlb-archi.png?raw=true "PaperLB Architecture")

## How does it work ?
PaperLB is implemented as a kubernetes "Operator": 
- Custom Resource Definitions
- Kubernetes Controllers that manage the Custom Resources and interact with your load balancer 

The idea is:

- You create a Kubernetes LoadBalancer type service and add some PaperLB annotations
- The controller notices the service and annotations and creates a "LoadBalancer" object
- The controller notices the "LoadBalancer" object and updates your network load balancer using the config data from the annotations + the service/nodes info

## Features
- Works with TCP or UDP L4 load balancers
- Adapters implemented:
  - Nginx: https://github.com/didil/nginx-lb-updater
- Updates load balancer configuration on:
  - Node updates
  - Service updates
- Deletes load balancer configuration on Service deletion

## Getting Started
You’ll need a kubernetes cluster to run against. You can use a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Usage
You can find the full example in the demo/ directory
Service example:
````yaml
apiVersion: v1
kind: Service
metadata:
  labels:
    app: k8s-pod-info-api
  name: k8s-pod-info-api-service
  #optional annotation to use a config different than the default config
  #annotations: 
  #  lb.paperlb.com/config-name: "my-special-config"  
spec:
  ports:
  - port: 5000
    protocol: TCP
    targetPort: 4000
  selector:
    app: k8s-pod-info-api
  type: LoadBalancer
  ````

LoadBalancerConfig example:
````yaml
apiVersion: lb.paperlb.com/v1alpha1
kind: LoadBalancerConfig
metadata:
  name: default-lb-config
  namespace: paperlb-system
spec:
  default: true
  httpUpdaterURL: "http://192.168.64.1:3000/api/v1/lb"
  host: "192.168.64.1"
  portRange: 
    low: 8100
    high: 8200
````

LoadBalancerConfig fields:
- `.spec.default`: "true" if this should be the default config, false otherwise
- `.spec.httpUpdaterURL`: URL where the http lb updater instance can be called. The API is explained here: https://github.com/didil/nginx-lb-updater#api
- `.spec.host`: Load Balancer Host
- `.spec.portRange`: The controller will select a load balancer port from this range  
- `.spec.portRange.low`: Lowest of the available ports on the load balancer 
- `.spec.portRange.high`: Highest of the available ports on the load balancer 

When you apply these manifests, a load balancer resource should be created. To get the load balancer connection info you can run:
````bash
$ k get loadbalancer k8s-pod-info-api-service
NAME                       HOST           PORT   PROTOCOL   TARGETCOUNT   STATUS
k8s-pod-info-api-service   192.168.64.1   8100   TCP        2             READY
````

Testing with Curl
````bash
$ curl -s 192.168.64.1:8100/api/v1/info|jq
  "pod": {
    "name": "k8s-pod-info-api-84dc7c9bdd-mz74t",
    "ip": "10.42.0.27",
    "namespace": "default",
    "serviceAccountName": "default"
  },
  "node": {
    "name": "k3s-local-server"
  }
}
````


### Run tests
To run tests:
```sh
make test
```

### Run locally
1. Install the CRDs into the cluster:

```sh
make install
```

2. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

3. You will need to run a load balancer instance and an API to allow the load balancer to be updated. You can use the example from this repository https://github.com/didil/nginx-lb-updater#run-locally 

4. The demo folder contains sample resource definitions to create a service and deployment. You can tweak them and run:
```sh
kubectl apply -f demo/ 
```

# Install PaperLB manifests to remote cluster
To install PaperLB CRDs and controllers, run:
````bash
kubectl apply -f https://raw.githubusercontent.com/didil/paperlb/v0.1.0/config/manifests/paperlb.yaml
````
The resources are created in the `paperlb-system` namespace.

## Contributing
Please feel free to open issues and / PRs if you'd like to contribute ! You can also get in touch at adil-paperlb@ledidil.com

## License

Copyright 2023 Adil H.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

