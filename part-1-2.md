# Step Two: Evolving the operator

I hope you enjoyed your coffee... Now, imagine it took you a couple of weeks and now you've decided to improve your operator by injecting a [sidecar](https://kubernetes.io/blog/2015/06/the-distributed-system-toolkit-patterns/#example-1-sidecar-containers) container that exposes Memcached server metrics to Prometheus on port `9150`.

We're going to use [this](https://github.com/prometheus/memcached_exporter) Promtheus exporter. Let's get started.

## Updating code to inject the Prometheus Exporter

Before we actually change the code let's update the versioning environment variables in `./settings.sh`

We're moving from:

```sh
export VERSION=0.0.1
```

to:

> **NOTE:** We need `FROM_VERSION` because we differentiate from the 1st version and the rest to add new bundle images instead to the previous index instead of create a new bundle index.

```sh
export FROM_VERSION=0.0.1
export VERSION=0.0.2
```

> **WARNING:** Don't forget to save `./settings.sh`!

Now, open file `./controllers/appservice_controller.go` and find 'Check if the deployment already exists, if not create a new one' it should around line 66. Until now the current code checks if the Deployment is already created and if not it creates it. We want to go a step further, if the deployment already exists then patch it (and add the sidecar container) if not create the deployment (also with the sidecar, although we do this later on, be patient)

We're moving from:

```go
    } else if err != nil {
        log.Error(err, "Failed to get Deployment")
        return ctrl.Result{}, err
    }
```

to:

```go
    } else if err == nil {
        from := &appsv1.Deployment{}
        if err = r.Client.Get(context.TODO(), types.NamespacedName{Name: appservice.Name, Namespace: appservice.Namespace}, from); err == nil {
            patch := r.deploymentForAppServicePatch(from)
            if err := r.Client.Patch(context.TODO(), from, patch); err != nil {
                return ctrl.Result{}, err
            }
        }
    } else {
        log.Error(err, "Failed to get Deployment")
        return ctrl.Result{}, err
    }
```

This should be the result.

```go
    // Check if the deployment already exists, if not create a new one
    found := &appsv1.Deployment{}
    err = r.Get(ctx, types.NamespacedName{Name: appservice.Name, Namespace: appservice.Namespace}, found)
    if err != nil && errors.IsNotFound(err) {
        // Define a new deployment
        dep := r.deploymentForAppService(appservice)
        log.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
        err = r.Create(ctx, dep)
        if err != nil {
            log.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
            return ctrl.Result{}, err
        }
        // Deployment created successfully - return and requeue
        return ctrl.Result{Requeue: true}, nil
    } else if err == nil {
        from := &appsv1.Deployment{}
        if err = r.Client.Get(context.TODO(), types.NamespacedName{Name: appservice.Name, Namespace: appservice.Namespace}, from); err == nil {
            patch := r.deploymentForAppServicePatch(from)
            if err := r.Client.Patch(context.TODO(), from, patch); err != nil {
                return ctrl.Result{}, err
            }
        }
    } else {
        log.Error(err, "Failed to get Deployment")
        return ctrl.Result{}, err
    }
```

Now let's change function `deploymentForAppService` for this, please find it and replace it with:

> **NOTE:** we're adding a new container called `exporter`

```go
// deploymentForAppService returns a appservice Deployment object
func (r *AppServiceReconciler) deploymentForAppService(m *gramophonev1.AppService) *appsv1.Deployment {
    ls := labelsForAppService(m.Name)
    replicas := m.Spec.Size

    dep := &appsv1.Deployment{
        ObjectMeta: metav1.ObjectMeta{
            Name:      m.Name,
            Namespace: m.Namespace,
        },
        Spec: appsv1.DeploymentSpec{
            Replicas: &replicas,
            Selector: &metav1.LabelSelector{
                MatchLabels: ls,
            },
            Template: corev1.PodTemplateSpec{
                ObjectMeta: metav1.ObjectMeta{
                    Labels: ls,
                },
                Spec: corev1.PodSpec{
                    Containers: []corev1.Container{
                        {
                            Image:   "memcached:1.4.36-alpine",
                            Name:    "memcached",
                            Command: []string{"memcached", "-m=64", "-o", "modern", "-v"},
                            Ports: []corev1.ContainerPort{{
                                ContainerPort: 11211,
                                Name:          "memcached",
                            }},
                        },
                        {
                            Image: "quay.io/prometheus/memcached-exporter:v0.7.0",
                            Name:  "exporter",
                            Ports: []corev1.ContainerPort{{
                                ContainerPort: 9150,
                                Name:          "exporter",
                            }},
                        },
                    },
                },
            },
        },
    }
    // Set AppService instance as the owner and controller
    ctrl.SetControllerReference(m, dep, r.Scheme)
    return dep
}
```

And create a new function `deploymentForAppServicePatch` after `deploymentForAppService` to deal with the patching.

```go
// deploymentForAppServicePatch returns a Patch
func (r *AppServiceReconciler) deploymentForAppServicePatch(current *appsv1.Deployment) client.Patch {
    patch := client.MergeFrom(current.DeepCopy())

    if len(current.Labels) <= 0 {
        current.Labels = map[string]string{
            "exporter":     "true",
            "exporterPort": "9150",
        }
    } else {
        current.Labels["exporter"] = "true"
        current.Labels["exporterPort"] = "9150"
    }

    memcachedExporterContainer := corev1.Container{
        Image: "quay.io/prometheus/memcached-exporter:v0.7.0",
        Name:  "exporter",
        Ports: []corev1.ContainerPort{{
            ContainerPort: 9150,
            Name:          "exporter",
        }},
    }

    // Add container
    if len(current.Spec.Template.Spec.Containers) > 0 {
        foundContainerIndex := -1
        for i, container := range current.Spec.Template.Spec.Containers {
            if container.Name == "exporter" {
                foundContainerIndex = i
            }
        }
        if foundContainerIndex != -1 {
            current.Spec.Template.Spec.Containers[foundContainerIndex] = memcachedExporterContainer
        } else {
            current.Spec.Template.Spec.Containers = append(current.Spec.Template.Spec.Containers, memcachedExporterContainer)
        }
    }

    return patch
}
```

## Testing our new code...

Before we actually do so, let's have another look to the standing status of our deployment.

```sh
 kubectl get pod -n operator-tests
NAME                                                      READY   STATUS    RESTARTS   AGE
appservice-sample-bf69d6cdf-rk4v5                         1/1     Running   1          2d20h
appservice-sample-bf69d6cdf-znj4w                         1/1     Running   1          2d20h
gramophone-operator-controller-manager-589d887cf7-whpt6   2/2     Running   4          2d20h
```

As you can see currently there's just 1 container per appservice pod. Compare to gramophone-operator-controller-manager it has 2.

Ok, time for testing. As we did before, you can run your code locally and see the result... but to avoid disturbances of the operator already running, let's scale down the operator to `zero` replicas.

```sh
$ kubectl scale deploy gramophone-operator-controller-manager  --replicas=0 -n operator-tests
deployment.apps/gramophone-operator-controller-manager scaled
```

And check the operator pod is gone.

```
$ kubectl get pod -n operator-tests
NAME                                READY   STATUS    RESTARTS   AGE
appservice-sample-bf69d6cdf-rk4v5   1/1     Running   1          2d20h
appservice-sample-bf69d6cdf-znj4w   1/1     Running   1          2d20h
```

Now let's run our operator code locally. Remember the deployment our operator created is already there... so our code should be patching.

> TIP: In another terminal run `watch kubectl get pod -n operator-tests` to see the changes live.

```sh
make run
```

If you run `kubectl get pod -n operator-tests` now you see 2/2 instead of 1/1. Please copy the name of one of the pods.

```sh
$ kubectl get pod -n operator-tests
NAME                                 READY   STATUS    RESTARTS   AGE
appservice-sample-6d88cb67b6-7nz6j   2/2     Running   0          9m44s
appservice-sample-6d88cb67b6-rgvfb   2/2     Running   0          9m37s
```

Let's check the exporter is working.

```sh
$ POD_NAME=appservice-sample-6d88cb67b6-7nz6j
$ kubectl exec -n operator-tests -it $POD_NAME -c exporter -- wget -q -O- http://localhost:9150/metrics
...
# HELP promhttp_metric_handler_requests_in_flight Current number of scrapes being served.
# TYPE promhttp_metric_handler_requests_in_flight gauge
promhttp_metric_handler_requests_in_flight 1
# HELP promhttp_metric_handler_requests_total Total number of scrapes by HTTP status code.
# TYPE promhttp_metric_handler_requests_total counter
promhttp_metric_handler_requests_total{code="200"} 7
promhttp_metric_handler_requests_total{code="500"} 0
promhttp_metric_handler_requests_total{code="503"} 0
```

Nice! Our code works locally.

What if it didn't... No problem, you can always debug... for instance with VSCode and the [golang extension](https://marketplace.visualstudio.com/items?itemName=golang.Go). You have to install the extension and then run the following commands. Finally, open the debugging area and click on the green arrow.

```sh
mkdir -p .vscode
cat << EOF > .voscode/launch.json
{
    // Use IntelliSense to learn about possible attributes.
    // Hover to view descriptions of existing attributes.
    // For more information, visit: https://go.microsoft.com/fwlink/?linkid=830387
    "version": "0.2.0",
    "configurations": [
        {
            "name": "Launch",
            "type": "go",
            "request": "launch",
            "mode": "auto",
            "program": "${workspaceFolder}/main.go",
            "env": {
                "DB_SCRIPTS_BASE_DIR": "."
            },
            "args": []
        }
    ]
}
EOF
```

So the code works, next stop.

## Running v0.0.2 as a normal deployment

Don't forget to reload environment... in any terminal you use... just in case:

```sh
. ./settings.sh
```

As we did before after running the code locally we have to build the operator image and run it from the operator deployment.

> **TIP:** Check that `VERSION` and `FROM_VERSION` are correct.

Let's build the operator image:

> **NOTE:** I hope you see something like this `Successfully tagged quay.io/cvicens/gramophone-operator-image:v0.0.2` at the end of `make docker--build` otherwise maybe you didn't notice the **TIP** ;-)

```sh
make docker-build
```

Let's push the image...

```sh
 make docker-push
```

Make sure you're not running the operator code locally and uninstall the operator.

```sh
$ kubectl operator uninstall gramophone-operator -n operator-tests
subscription "gramophone-operator" deleted
clusterserviceversion "gramophone-operator.v0.0.1" deleted
clusterrole "gramophone-operator-metrics-reader" deleted
role "gramophone-operator.v0.0.1-h9gpt" deleted
rolebinding "gramophone-operator.v0.0.1-h9gpt-default-tstvj" deleted
clusterrole "gramophone-operator.v0.0.1-ww9ft" deleted
operator "gramophone-operator" uninstalled
```

Let's deploy the operator as we did with version 0.0.1

```sh
$ make deploy
go: creating new go.mod: module tmp
go: finding sigs.k8s.io/controller-tools/cmd/controller-gen v0.3.0
go: finding sigs.k8s.io v0.3.0
go: finding sigs.k8s.io/controller-tools/cmd v0.3.0
/Users/cvicensa/go/bin/controller-gen "crd:trivialVersions=true" rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases
cd config/manager && /usr/local/bin/kustomize edit set image controller=quay.io/cvicens/gramophone-operator-image:v0.0.2
/usr/local/bin/kustomize build config/default | kubectl apply -f -
namespace/gramophone-operator-system created
Warning: kubectl apply should be used on resource created by either kubectl create --save-config or kubectl apply
customresourcedefinition.apiextensions.k8s.io/appservices.gramophone.atarazana.com configured
role.rbac.authorization.k8s.io/gramophone-operator-leader-election-role created
clusterrole.rbac.authorization.k8s.io/gramophone-operator-manager-role created
clusterrole.rbac.authorization.k8s.io/gramophone-operator-proxy-role created
clusterrole.rbac.authorization.k8s.io/gramophone-operator-metrics-reader created
rolebinding.rbac.authorization.k8s.io/gramophone-operator-leader-election-rolebinding created
clusterrolebinding.rbac.authorization.k8s.io/gramophone-operator-manager-rolebinding created
clusterrolebinding.rbac.authorization.k8s.io/gramophone-operator-proxy-rolebinding created
service/gramophone-operator-controller-manager-metrics-service created
deployment.apps/gramophone-operator-controller-manager created
```

Let's check image is version 0.0.2:

```sh
$ kubectl get deploy gramophone-operator-controller-manager -o jsonpath='{.spec.template.spec.containers[0].image}' -n $PROJECT_NAME && echo
quay.io/cvicens/gramophone-operator-image:v0.0.2
```

Now let's create a sample AppService:

> **NOTE:** This test is different from the one we did when we run the code locally because there's no previous AppService and hence there's no previous Memcached deployment to update.

```sh
$ kubectl apply -n $PROJECT_NAME -f ./config/samples/gramophone_v1_appservice.yaml 
appservice.gramophone.atarazana.com/appservice-sample created
```

If it all went right there should be 2 pods with 2 containers plus the pod operator:

```sh
$ kubectl get pod -n $PROJECT_NAME 
NAME                                                      READY   STATUS    RESTARTS   AGE
appservice-sample-6d88cb67b6-gv62q                        2/2     Running   0          28s
appservice-sample-6d88cb67b6-jhspm                        2/2     Running   0          28s
gramophone-operator-controller-manager-6fdbfb88d8-8v994   2/2     Running   0          12m
```

Again if we run the next commands (don't forget to replace the name of the pod with your own):

```sh
$ POD_NAME=appservice-sample-6d88cb67b6-gv62q
$ kubectl exec -n $PROJECT_NAME -it $POD_NAME -c exporter -- wget -q -O- http://localhost:9150/metrics
...
# HELP promhttp_metric_handler_requests_in_flight Current number of scrapes being served.
# TYPE promhttp_metric_handler_requests_in_flight gauge
promhttp_metric_handler_requests_in_flight 1
# HELP promhttp_metric_handler_requests_total Total number of scrapes by HTTP status code.
# TYPE promhttp_metric_handler_requests_total counter
promhttp_metric_handler_requests_total{code="200"} 0
promhttp_metric_handler_requests_total{code="500"} 0
promhttp_metric_handler_requests_total{code="503"} 0
```

Great... now we now the code works starting from an existing AppService... and also that the operator works from scratch with no previous AppService.


Next stop, create the bundle for the current version (0.0.2).

## Create an operator bundle and it's corresponding image for version 0.0.2

This should be easy... just run the following to regenerate the bundle for the new version... wait, wait, I forgot something crucial here... Version 0.0.2 replaces 0.0.1 this should be specified somewhere... specifically in the base CSV. Let's do it.

Open file `./config/manifests/bases/${OPERATOR_NAME}.clusterserviceversion.yaml`, and add `replaces: ${OPERATOR_NAME}.v0.0.1` right underneath `version: 0.0.0`, **pay attention to indentation**. The end result should look like this:

> **WARNING:** In my case `${OPERATOR_NAME}.v0.0.1` === `gramophone-operator.v0.0.1` in your case it depends on if you changed your `./settings.sh` file or not.

```yaml
...
spec:
  ...
  provider:
    name: Atarazana Inc.
    url: https://es.wikipedia.org/wiki/Astillero_naval
  version: 0.0.0
  replaces: gramophone-operator.v0.0.1
```

Now you can run:

```sh
make bundle
```

Check that this returns `${OPERATOR_NAME}.v0.0.1`

```sh
$ grep replaces ./bundle/manifests/gramophone-operator.clusterserviceversion.yaml
  replaces: gramophone-operator.v0.0.1
```

Also that this returns `  name: ${OPERATOR_NAME}.v0.0.2`

```sh
$ grep ${OPERATOR_NAME}.v${VERSION} ./bundle/manifests/gramophone-operator.clusterserviceversion.yaml
  name: gramophone-operator.v0.0.2
```

Well now we have to build and push the bundle image:

```
make bundle-build
make bundle-push
```

## Create and push the new Bundle Index (0.0.2)

As we advanced before we use `FROM_VERSION` so that we can understand of we have to create a new Index from scratch or start from a previous bundle index to create the new one. Because `FROM_VERSION` is defined the makefile target will create index 0.0.2 based on (from) 0.0.1

```sh
$  make index-build
echo "FROM_VERSION 0.0.1"
FROM_VERSION 0.0.1
opm -u docker index add --bundles quay.io/cvicens/gramophone-operator-bundle:v0.0.2 --from-index quay.io/cvicens/gramophone-operator-index:v0.0.1 --tag quay.io/cvicens/gramophone-operator-index:v0.0.2
...
INFO[0004] Generating dockerfile                         bundles="[quay.io/cvicens/gramophone-operator-bundle:v0.0.2]"
INFO[0004] writing dockerfile: index.Dockerfile895643308  bundles="[quay.io/cvicens/gramophone-operator-bundle:v0.0.2]"
INFO[0004] running docker build                          bundles="[quay.io/cvicens/gramophone-operator-bundle:v0.0.2]"
INFO[0004] [docker build -f index.Dockerfile895643308 -t quay.io/cvicens/gramophone-operator-index:v0.0.2 .]  bundles="[quay.io/cvicens/gramophone-operator-bundle:v0.0.2]"
```

Let's push our new index:

```sh
make index-push
```

## Update the CatalogSource to point to the new Bundle Index

Let's do some cleaning before we test our new bundle index.

```sh
make catalog-undeploy
make undeploy
```

Now let's deploy the previous version of the CatalogSource, the one pointing to the previous version of the bundle index. This means we'll be using `FROM_VERSION` ;-)

```sh
$ make catalog-deploy-prev
sed "s|BUNDLE_INDEX_IMG|quay.io/cvicens/gramophone-operator-index:v0.0.1|" ./config/catalog/catalog-source.yaml | kubectl apply -n olm -f -
catalogsource.operators.coreos.com/atarazana-catalog created
```

Let's check if the operator is available to be installed.

```sh
$ kubectl operator list-available ${OPERATOR_NAME}
NAME                 CATALOG              CHANNEL  LATEST CSV                  AGE
gramophone-operator  Atarazana Operators  alpha    gramophone-operator.v0.0.1  63s
```

Let's install the operator in our test namespace.

```sh
$ kubectl operator install ${OPERATOR_NAME} -n operator-tests
subscription "gramophone-operator" created
operator "gramophone-operator" installed; installed csv is "gramophone-operator.v0.0.1"
```

Now let's create an AppService object.

```sh
$ kubectl apply -f ./config/samples/gramophone_v1_appservice.yaml -n operator-tests
appservice.gramophone.atarazana.com/appservice-sample created
```

Ok, as expected 2 pods because size is 2 and 1 container per pod because it's version 0.0.1.

```sh
$ kubectl get pod -n operator-tests
NAME                                                      READY   STATUS    RESTARTS   AGE
appservice-sample-bf69d6cdf-7m2bd                         1/1     Running   0          23s
appservice-sample-bf69d6cdf-rtcpn                         1/1     Running   0          22s
gramophone-operator-controller-manager-589d887cf7-6wrw6   2/2     Running   0          77s
```

Now the moment of truth. Let's update the CatalogSource to point to the new bundle index. Fingers crossed! (Well not needed)

```sh
make catalog-deploy
```

Wait 5 secs or so and list the operators and you'll see that there's a new version (CURRENT CSV) and the status is `UpgradePending`.

```sh
$ kubectl operator list 
PACKAGE              SUBSCRIPTION         INSTALLED CSV               CURRENT CSV                 STATUS          AGE
gramophone-operator  gramophone-operator  gramophone-operator.v0.0.1  gramophone-operator.v0.0.2  UpgradePending  4m43s
```

So no changes so far, we have to upgrade to actually upgrade the image of the operator and move to the next version.

```sh
$ kubectl operator upgrade $OPERATOR_NAME -n operator-tests
operator "gramophone-operator" upgraded; installed csv is "gramophone-operator.v0.0.2"
```

## Final tests

Finally, let's check the result. And yes... there it is the second container!

```sh
$ kubectl get pod -n operator-tests
NAME                                                      READY   STATUS    RESTARTS   AGE
appservice-sample-6d88cb67b6-7r9fl                        2/2     Running   0          10s
appservice-sample-6d88cb67b6-xxsgx                        2/2     Running   0          11s
gramophone-operator-controller-manager-75c9f846b8-pqlkv   2/2     Running   0          31s
```

You made it.

## Final thoughts

This is just an example that tries to cover a lot of land... I think it should be enough to start enjoying the Operator Framework. There's still a lot to learn so stay in touch!