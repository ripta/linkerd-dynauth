# linkerd-dynauth

CRD to allow dynamic namespace selector in server authorizations, somewhat
implementing https://github.com/linkerd/linkerd2/issues/9419

After making changes to the API under `api/v1alpha1`, regenerate manifest:

```
make generate manifests
```

## Building

To build and push your image to the location specified by `IMG`:

```sh
make docker-build docker-push IMG=<some-registry>/linkerd-dynauth2:tag
```

## Installing

To install the CRDs into the cluster:

```sh
make install
```

To deploy the Manager to the cluster with the image specified by `IMG`:

```sh
make deploy IMG=<some-registry>/linkerd-dynauth2:tag
```

**NOTE**: If you encounter RBAC errors, you may need to grant yourself
cluster-admin privileges or be logged in as admin.

## Uninstall

To delete the APIs(CRDs) from the cluster:

```sh
make uninstall
```

To undeploy the controller from the cluster:

```sh
make undeploy
```

## Project Distribution

Following are the steps to build the installer and distribute this project to users.

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/linkerd-dynauth2:tag
```

NOTE: The makefile target mentioned above generates an 'install.yaml' file in
the dist directory. This file contains all the resources built with Kustomize,
which are necessary to install this project without its dependencies.

2. Using the installer

Users can just run:

```sh
kubectl apply -f https://raw.githubusercontent.com/ripta/linkerd-dynauth2/main/dist/install.yaml
```
