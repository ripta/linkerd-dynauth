/*
Copyright 2022 Ripta Pasay.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"hash/fnv"
	"net"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	serverauthorizationv1beta1 "github.com/linkerd/linkerd2/controller/gen/apis/serverauthorization/v1beta1"

	linkerddynauthv1alpha1 "github.com/ripta/linkerd-dynauth/api/v1alpha1"
	"github.com/ripta/linkerd-dynauth/util"
)

const ServerAuthorizationMatchLabelKey = "linkerd-dynauth.k.r8y.net/dsa-request-key"

var fieldOwner = client.FieldOwner("linkerd-dynauth")

// DynamicServerAuthorizationReconciler reconciles a DynamicServerAuthorization object
type DynamicServerAuthorizationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=linkerd-dynauth.k.r8y.net,resources=dynamicserverauthorizations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=linkerd-dynauth.k.r8y.net,resources=dynamicserverauthorizations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=linkerd-dynauth.k.r8y.net,resources=dynamicserverauthorizations/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch
//+kubebuilder:rbac:groups=policy.linkerd.io/v1beta1,resources=serverauthorizations,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the DynamicServerAuthorization object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *DynamicServerAuthorizationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)
	l.Info("reconciliation start", "object_key", req)

	dsa := &linkerddynauthv1alpha1.DynamicServerAuthorization{}
	if err := r.Get(ctx, req.NamespacedName, dsa); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	lsaLabels := map[string]string{}
	for k, v := range dsa.Spec.CommonMetadata.Labels {
		lsaLabels[k] = v
	}
	lsaLabels[ServerAuthorizationMatchLabelKey] = req.Name

	lsaAnnotations := map[string]string{}
	for k, v := range dsa.Spec.CommonMetadata.Annotations {
		lsaAnnotations[k] = v
	}

	if dsa.Spec.Client.Healthcheck {
		nodeList := &corev1.NodeList{}
		if err := r.List(ctx, nodeList); err != nil {
			return ctrl.Result{}, err
		}

		cidrs := []*serverauthorizationv1beta1.Cidr{}
		for _, node := range nodeList.Items {
			for _, addr := range node.Status.Addresses {
				if addr.Type != corev1.NodeInternalIP && addr.Type != corev1.NodeExternalIP {
					continue
				}

				ip := net.ParseIP(addr.Address)
				if ip == nil {
					continue
				}

				cidrs = append(cidrs, &serverauthorizationv1beta1.Cidr{
					Cidr: fmt.Sprintf("%s/32", ip.String()),
				})
			}
		}

		sort.Slice(cidrs, func(i, j int) bool {
			return cidrs[i].Cidr < cidrs[j].Cidr
		})

		lsa := serverauthorizationv1beta1.ServerAuthorization{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   dsa.Namespace,
				Name:        dsa.Name + "-healthcheck",
				Labels:      lsaLabels,
				Annotations: lsaAnnotations,
			},
			Spec: serverauthorizationv1beta1.ServerAuthorizationSpec{
				Server: *dsa.Spec.Server.DeepCopy(),
				Client: serverauthorizationv1beta1.Client{
					Networks: cidrs,
				},
			},
		}

		if err := controllerutil.SetControllerReference(dsa, &lsa, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}

		id := types.NamespacedName{
			Namespace: lsa.Namespace,
			Name:      lsa.Name,
		}

		found := &serverauthorizationv1beta1.ServerAuthorization{}
		if err := r.Get(ctx, id, found); err != nil {
			if errors.IsNotFound(err) {
				// CREATE LSA
				l.Info("creating server authorization for healthcheck grant", "server_authorization_name", lsa.Name)
				if err := r.Create(ctx, &lsa, fieldOwner); err != nil {
					return ctrl.Result{Requeue: true}, err
				}
			} else {
				return ctrl.Result{}, err
			}
		} else {
			// UPDATE LSA
			patch := client.MergeFrom(found.DeepCopy())
			found.Annotations = lsa.Annotations
			found.Labels = lsa.Labels
			found.Spec = lsa.Spec
			l.Info("patching server authorization for healthcheck grant", "request_key", req, "server_authorization_name", found.Name)
			if err := r.Patch(ctx, found, patch, fieldOwner); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		id := types.NamespacedName{
			Namespace: dsa.Namespace,
			Name:      dsa.Name + "-healthcheck",
		}

		found := &serverauthorizationv1beta1.ServerAuthorization{}
		if err := r.Get(ctx, id, found); err != nil {
			if !errors.IsNotFound(err) {
				return ctrl.Result{}, err
			}
		} else {
			l.Info("deleting server authorization for healthcheck grant", "request_key", req, "server_authorization_nme", found.Name)
			if err := r.Delete(ctx, found); err != nil {
				if !errors.IsNotFound(err) {
					return ctrl.Result{}, err
				}
			}
		}
	}

	if dsa.Spec.Client.MeshTLS == nil {
		return ctrl.Result{}, nil
	}

	// Calculate the list of linkerd server authorizations that should exist
	expectedLSA := []serverauthorizationv1beta1.ServerAuthorization{}
	expectedNames := map[string]struct{}{}
	for _, dm := range dsa.Spec.Client.MeshTLS.ServiceAccounts {
		sel, err := metav1.LabelSelectorAsSelector(dm.NamespaceSelector)
		if err != nil {
			return ctrl.Result{}, err
		}

		nsList := &corev1.NamespaceList{}
		nsOpts := []client.ListOption{
			client.MatchingLabelsSelector{
				Selector: sel,
			},
		}
		if err := r.List(ctx, nsList, nsOpts...); err != nil {
			return ctrl.Result{}, err
		}

		lms := []*serverauthorizationv1beta1.ServiceAccountName{}
		for _, ns := range nsList.Items {
			lms = append(lms, &serverauthorizationv1beta1.ServiceAccountName{
				Namespace: ns.Name,
				Name:      dm.Name,
			})
		}

		var mtls *serverauthorizationv1beta1.MeshTLS
		if len(lms) > 0 {
			mtls = &serverauthorizationv1beta1.MeshTLS{
				ServiceAccounts: lms,
			}
		}

		lsa := serverauthorizationv1beta1.ServerAuthorization{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   dsa.Namespace,
				Name:        dsa.Name + "-" + ComputeHashServiceAccountSelector(dm),
				Labels:      lsaLabels,
				Annotations: lsaAnnotations,
			},
			Spec: serverauthorizationv1beta1.ServerAuthorizationSpec{
				Server: *dsa.Spec.Server.DeepCopy(),
				Client: serverauthorizationv1beta1.Client{
					MeshTLS: mtls,
				},
			},
		}

		if err := controllerutil.SetControllerReference(dsa, &lsa, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}

		expectedLSA = append(expectedLSA, lsa)
		expectedNames[lsa.Name] = struct{}{}
	}

	for _, lsa := range expectedLSA {
		id := types.NamespacedName{
			Namespace: lsa.Namespace,
			Name:      lsa.Name,
		}

		found := &serverauthorizationv1beta1.ServerAuthorization{}
		if err := r.Get(ctx, id, found); err != nil {
			if errors.IsNotFound(err) {
				// CREATE LSA
				if err := r.Create(ctx, &lsa, fieldOwner); err != nil {
					return ctrl.Result{Requeue: true}, err
				}
				continue
			}

			return ctrl.Result{}, err
		}

		// UPDATE LSA
		patch := client.MergeFrom(found.DeepCopy())
		found.Annotations = lsa.Annotations
		found.Labels = lsa.Labels
		found.Spec = lsa.Spec
		l.Info("patching server authorization for service account grant", "request_key", req, "server_authorization_name", found.Name, "server_authorization_spec", found.Spec)
		if err := r.Patch(ctx, found, patch, fieldOwner); err != nil {
			return ctrl.Result{}, err
		}
	}

	lsaList := &serverauthorizationv1beta1.ServerAuthorizationList{}
	lsaOpts := []client.ListOption{
		client.InNamespace(req.Namespace),
		client.MatchingLabels{
			ServerAuthorizationMatchLabelKey: req.Name,
		},
	}
	if err := r.List(ctx, lsaList, lsaOpts...); err != nil {
		return ctrl.Result{}, err
	}

	for _, lsa := range lsaList.Items {
		if _, ok := expectedNames[lsa.Name]; ok {
			continue
		}

		// DELETE LSA
		l.Info("deleting server authorization", "request_key", req, "server_authorization", lsa.ObjectMeta)
		if err := r.Delete(ctx, &lsa); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DynamicServerAuthorizationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	o := controller.Options{
		MaxConcurrentReconciles: 5,
	}

	ctx := context.TODO()
	mgr.GetFieldIndexer().IndexField(
		ctx,
		&linkerddynauthv1alpha1.DynamicServerAuthorization{},
		"spec.client.healthcheck",
		r.nodeToHealthcheckAuthorizationIndexer,
	)
	mgr.GetFieldIndexer().IndexField(
		ctx,
		&linkerddynauthv1alpha1.DynamicServerAuthorization{},
		"spec.client.meshTLS.serviceAccounts.namespaceSelector.labelKey",
		func(obj client.Object) []string {
			dsa, ok := obj.(*linkerddynauthv1alpha1.DynamicServerAuthorization)
			if !ok {
				return nil
			}

			if dsa.Spec.Client.MeshTLS == nil {
				return nil
			}

			values := []string{}
			for _, sa := range dsa.Spec.Client.MeshTLS.ServiceAccounts {
				if sa.NamespaceSelector == nil {
					continue
				}

				for k := range sa.NamespaceSelector.MatchLabels {
					values = append(values, k)
				}

				for _, exp := range sa.NamespaceSelector.MatchExpressions {
					if exp.Operator == metav1.LabelSelectorOpExists || exp.Operator == metav1.LabelSelectorOpIn {
						values = append(values, exp.Key)
					}
				}
			}

			return values
		},
	)

	return ctrl.NewControllerManagedBy(mgr).
		For(&linkerddynauthv1alpha1.DynamicServerAuthorization{}).
		Owns(&serverauthorizationv1beta1.ServerAuthorization{}).
		Watches(
			&source.Kind{
				Type: &corev1.Namespace{},
			},
			handler.EnqueueRequestsFromMapFunc(r.namespaceToEnqueueRequestMapper),
		).
		Watches(
			&source.Kind{
				Type: &corev1.Node{},
			},
			handler.EnqueueRequestsFromMapFunc(r.nodeToHealthcheckAuthorizationMapper),
			builder.WithPredicates(&nodeIPFilter{}),
		).
		WithOptions(o).
		Complete(r)
}

func (r *DynamicServerAuthorizationReconciler) nodeToHealthcheckAuthorizationIndexer(obj client.Object) []string {
	dsa, ok := obj.(*linkerddynauthv1alpha1.DynamicServerAuthorization)
	if !ok {
		return nil
	}

	if !dsa.Spec.Client.Healthcheck {
		return nil
	}

	return []string{"true"}
}

func (r *DynamicServerAuthorizationReconciler) nodeToHealthcheckAuthorizationMapper(obj client.Object) []reconcile.Request {
	ctx := context.TODO()

	if _, ok := obj.(*corev1.Node); !ok {
		log.Log.Error(fmt.Errorf("unexpected object type %T, when expecting corev1.Node", obj), "in HealthcheckAuthorizer mapper")
		return nil
	}

	hca := client.MatchingFields{
		"spec.client.healthcheck": "true",
	}

	dsal := &linkerddynauthv1alpha1.DynamicServerAuthorizationList{}
	if err := r.List(ctx, dsal, hca); err != nil {
		log.Log.Error(err, "listing DynamicServerAuthorization")
		return nil
	}

	reqs := []reconcile.Request{}
	for _, dsa := range dsal.Items {
		if !dsa.Spec.Client.Healthcheck {
			continue
		}

		reqs = append(reqs, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: dsa.Namespace,
				Name:      dsa.Name,
			},
		})
	}

	return reqs
}

func (r *DynamicServerAuthorizationReconciler) namespaceToEnqueueRequestMapper(obj client.Object) []reconcile.Request {
	ctx := context.TODO()

	ns, ok := obj.(*corev1.Namespace)
	if !ok {
		log.Log.Error(fmt.Errorf("unexpected object type %T, when expecting corev1.Namespace", obj), "in EnqueueRequest mapper")
		return nil
	}
	nsLabelSet := labels.Set(ns.Labels)

	// log.Log.Info("reconciling namespace", "object_labels", nsLabelSet)

	dsal := &linkerddynauthv1alpha1.DynamicServerAuthorizationList{}
	if err := r.List(ctx, dsal); err != nil {
		log.Log.Error(err, "listing DynamicServerAuthorizationList")
		return nil
	}

	reqs := []reconcile.Request{}
	for _, dsa := range dsal.Items {
		if dsa.Spec.Client.MeshTLS == nil {
			continue
		}

		for _, msa := range dsa.Spec.Client.MeshTLS.ServiceAccounts {
			sel, err := metav1.LabelSelectorAsSelector(msa.NamespaceSelector)
			if err != nil {
				continue
			}

			if sel.Matches(nsLabelSet) {
				reqs = append(reqs, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: dsa.Namespace,
						Name:      dsa.Name,
					},
				})
			}
		}
	}

	if len(reqs) > 0 {
		log.Log.Info("triggering reconciliation", "namespace", ns.Name, "reconciliation_count", len(reqs), "reconciliation_keys", reqs)
	}

	return reqs
}

func ComputeHashServiceAccountSelector(selector *linkerddynauthv1alpha1.ServiceAccountSelector) string {
	hasher := fnv.New32a()
	util.DeepHashObject(hasher, *selector)
	return rand.SafeEncodeString(fmt.Sprint(hasher.Sum32()))
}
