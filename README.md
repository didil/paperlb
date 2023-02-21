# PaperLB

![Alt text](logo-color.png?raw=true "PaperLB Logo")

A kubernetes network load balancer operator implementation.


**THIS SOFTWARE IS WORK IN PROGRESS / ALPHA RELEASE AND IS NOT MEANT FOR USAGE IN PRODUCTION SYSTEMS**

## What is PaperLB ?
Vanilla kubernetes does not come with a LoadBalancer Service implementation. If you create a LoadBalancer Service in a self-hosted cluster setup, its status will remain "PENDING".

PaperLB allows you to use an external L4 load balancer (an nginx server for example) in front of your cluster services. 

![Alt text](paperlb-archi.png?raw=true "PaperLB Architecture")

## How does it work ?
PaperLB is implemented as a kubernetes "Operator": 
- Custom Resource Definitions
- Kubernetes Controllers that manage the Custom Resources and interact with your load balancer 

The idea is:

- You create a kubernetes LoadBalancer type service and add some paperlb annotations
- The controller notices the service and annotations and creates a "LoadBalancer" object
- The controller notices the "LoadBalancer" object and updates your actual load balancer using the config from the annotations from and the service/nodes info


## Getting Started
You’ll need a kubernetes cluster to run against. You can use a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Usage
Service example:
````yaml
apiVersion: v1
kind: Service
metadata:
  labels:
    app: k8s-pod-info-api
  name: k8s-pod-info-api-service
  annotations:
    lb.paperlb.com/http-updater-url: "http://192.168.64.1:3000/api/v1/lb"
    lb.paperlb.com/load-balancer-host: "192.168.64.1"
    lb.paperlb.com/load-balancer-port: "8100"
    lb.paperlb.com/load-balancer-protocol: "TCP"
spec:
  ports:
  - port: 5000
    protocol: TCP
    targetPort: 4000
  selector:
    app: k8s-pod-info-api
  type: LoadBalancer
````

Annotations: 
- `lb.paperlb.com/http-updater-url`: URL where the http lb updater instance can be called. The API is explained here: https://github.com/didil/nginx-lb-updater#api
- `lb.paperlb.com/load-balancer-host`: Load Balancer Host
- `lb.paperlb.com/load-balancer-port`: Load Balancer Port
- `lb.paperlb.com/load-balancer-protocol`: Load Balancer Protocol (`TCP` or `UDP`)
 


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


**NOTE:** You can also run this in one step by running: `make install run`

### Uninstall CRDs
To delete the CRDs from the cluster:

```sh
make uninstall
```

## Contributing
Please feel free to open issues and / PRs if you'd like to contribute ! You can also get in touch at adil-paperlb@ledidil.com


### How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/),
which provide a reconcile function responsible for synchronizing resources until the desired state is reached on the cluster.



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

