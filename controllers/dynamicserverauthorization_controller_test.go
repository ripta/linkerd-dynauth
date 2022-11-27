package controllers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	serverauthorizationv1beta1 "github.com/linkerd/linkerd2/controller/gen/apis/serverauthorization/v1beta1"

	linkerddynauthv1alpha1 "github.com/ripta/linkerd-dynauth/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Dynamic Server Authorization controller", func() {
	Context("test healthcheck", func() {
		const DSAName = "foobar-healthcheck-grant"

		ctx := context.Background()

		tn := types.NamespacedName{
			Namespace: DSAName,
			Name:      DSAName,
		}
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: DSAName,
				Name:      DSAName,
			},
		}

		BeforeEach(func() {
			By("Creating the namespace in which tests are performed")
			Expect(k8sClient.Create(ctx, ns)).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			By("Deleting the namespace in which test are performed")
			_ = k8sClient.Delete(ctx, ns)
		})

		It("should successfully reconcile a dynamic server authorization resource", func() {
			By("Creating the custom resource")
			dsa := &linkerddynauthv1alpha1.DynamicServerAuthorization{}
			if err := k8sClient.Get(ctx, tn, dsa); err != nil && errors.IsNotFound(err) {
				dsa := &linkerddynauthv1alpha1.DynamicServerAuthorization{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: DSAName,
						Name:      DSAName,
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
			}, time.Minute, time.Second).Should(Succeed())

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
					Namespace: DSAName,
					Name:      DSAName + "-healthcheck",
				}

				found := &serverauthorizationv1beta1.ServerAuthorization{}
				return k8sClient.Get(ctx, saz, found)
			}, time.Minute, time.Second).Should(Succeed())
		})
	})
})
