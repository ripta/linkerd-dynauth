package controllers

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/util/rand"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	serverauthorizationv1beta1 "github.com/linkerd/linkerd2/controller/gen/apis/serverauthorization/v1beta1"

	linkerddynauthv1alpha1 "github.com/ripta/linkerd-dynauth/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Dynamic Server Authorization controller", func() {
	Context("test resource reconciliation", func() {
		var ResourceName string
		var tn types.NamespacedName

		ctx := context.Background()

		BeforeEach(func() {
			ResourceName = fmt.Sprintf("foobar-grant-%d", rand.Int())

			tn = types.NamespacedName{
				Namespace: ResourceName,
				Name:      ResourceName,
			}

			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ResourceName,
					Name:      ResourceName,
				},
			}

			By("Creating the namespace in which tests are performed")
			Expect(k8sClient.Create(ctx, ns)).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			dsaList := &linkerddynauthv1alpha1.DynamicServerAuthorizationList{}
			Expect(k8sClient.List(ctx, dsaList, client.InNamespace(ResourceName))).NotTo(HaveOccurred())

			ns := &corev1.Namespace{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name: ResourceName,
			}, ns)).NotTo(HaveOccurred())

			By("Deleting the namespace in which test are performed")
			_ = k8sClient.Delete(ctx, ns)
		})

		It("should successfully reconcile server authorization resources for service accounts", func() {
			By("Creating the custom resource")
			dsa := &linkerddynauthv1alpha1.DynamicServerAuthorization{}
			if err := k8sClient.Get(ctx, tn, dsa); err != nil && errors.IsNotFound(err) {
				dsa := &linkerddynauthv1alpha1.DynamicServerAuthorization{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: ResourceName,
						Name:      ResourceName,
					},
					Spec: linkerddynauthv1alpha1.DynamicServerAuthorizationSpec{
						Server: serverauthorizationv1beta1.Server{
							Name: "test",
						},
						Client: linkerddynauthv1alpha1.Client{
							MeshTLS: &linkerddynauthv1alpha1.MeshTLS{
								ServiceAccounts: []*linkerddynauthv1alpha1.ServiceAccountSelector{
									{
										Name: "default",
										NamespaceSelector: &metav1.LabelSelector{
											MatchExpressions: []metav1.LabelSelectorRequirement{
												{
													Key:      "group.k.r8y.net/name",
													Operator: "In",
													Values:   []string{"foo", "bar"},
												},
											},
										},
									},
								},
							},
						},
					},
				}

				Expect(k8sClient.Create(ctx, dsa)).NotTo(HaveOccurred())
			}

			By("Checking for the custom resource")
			Eventually(func() error {
				found := &linkerddynauthv1alpha1.DynamicServerAuthorization{}
				return k8sClient.Get(ctx, tn, found)
			}, 10*time.Second, time.Second).Should(Succeed())

			By("Reconciling the custom resource")
			r := &DynamicServerAuthorizationReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: tn,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking if a server authorization was created")
			Eventually(func() (int, error) {
				sazList := &serverauthorizationv1beta1.ServerAuthorizationList{}
				if err := k8sClient.List(ctx, sazList); err != nil {
					return 0, err
				}
				return len(sazList.Items), nil
			}, 10*time.Second, time.Second).Should(Equal(1))
		})

		It("should successfully reconcile server authorization resources for healthcheck grant", func() {
			By("Creating the custom resource")
			dsa := &linkerddynauthv1alpha1.DynamicServerAuthorization{}
			if err := k8sClient.Get(ctx, tn, dsa); err != nil && errors.IsNotFound(err) {
				dsa := &linkerddynauthv1alpha1.DynamicServerAuthorization{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: ResourceName,
						Name:      ResourceName,
					},
					Spec: linkerddynauthv1alpha1.DynamicServerAuthorizationSpec{
						Server: serverauthorizationv1beta1.Server{
							Name: "test",
						},
						Client: linkerddynauthv1alpha1.Client{
							Healthcheck: true,
						},
					},
				}

				Expect(k8sClient.Create(ctx, dsa)).NotTo(HaveOccurred())
			}

			By("Checking for the custom resource")
			Eventually(func() error {
				found := &linkerddynauthv1alpha1.DynamicServerAuthorization{}
				return k8sClient.Get(ctx, tn, found)
			}, 10*time.Second, time.Second).Should(Succeed())

			By("Reconciling the custom resource")
			r := &DynamicServerAuthorizationReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: tn,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking if a server authorization was created")
			Eventually(func() error {
				saz := types.NamespacedName{
					Namespace: ResourceName,
					Name:      ResourceName + "-healthcheck",
				}

				found := &serverauthorizationv1beta1.ServerAuthorization{}
				return k8sClient.Get(ctx, saz, found)
			}, 10*time.Second, time.Second).Should(Succeed())
		})
	})
})
