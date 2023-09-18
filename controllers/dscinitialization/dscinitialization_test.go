package dscinitialization

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	dsci "github.com/opendatahub-io/opendatahub-operator/v2/apis/dscinitialization/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	authv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	applicationName      = "test-dsci"
	workingNamespace     = "default"
	applicationNamespace = "test-application-ns"
	monitoringNamespace  = "test-monitoring-ns"
	configmapName        = "odh-common-config"
	readyPhase           = "Ready"
)

var _ = Describe("DataScienceCluster initialization", Ordered, func() {
	Context("Should create default resources", func() {
		ctx := context.Background()
		It("Should create an instance of DSCI", func() {
			desiredDsci := &dsci.DSCInitialization{
				TypeMeta: metav1.TypeMeta{
					Kind:       "DSCInitialization",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      applicationName,
					Namespace: workingNamespace,
				},
				Spec: dsci.DSCInitializationSpec{
					ApplicationsNamespace: applicationNamespace,
					Monitoring: dsci.Monitoring{
						Namespace:       monitoringNamespace,
						ManagementState: operatorv1.Managed,
					},
				},
			}
			Expect(k8sClient.Create(ctx, desiredDsci)).Should(Succeed())
			foundDsci := &dsci.DSCInitialization{}
			Eventually(func() bool {
				k8sClient.Get(ctx, client.ObjectKey{
					Name:      applicationName,
					Namespace: workingNamespace,
				}, foundDsci)
				return foundDsci.Status.Phase == readyPhase
			}, timeout, interval).Should(BeTrue())
			Expect(foundDsci.Name).To(Equal(applicationName))
		})
		It("Should create the specified application namespace", func() {
			foundApplicationNamespace := &corev1.Namespace{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: applicationNamespace}, foundApplicationNamespace)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(foundApplicationNamespace.Name).To(Equal(applicationNamespace))
			expectedLabels := map[string]string{
				"kubernetes.io/metadata.name":        applicationNamespace,
				"opendatahub.io/generated-namespace": "true",
				"pod-security.kubernetes.io/enforce": "baseline",
			}
			Expect(foundApplicationNamespace.Labels).To(Equal(expectedLabels))
		})
		It("Should create the specified monitoring namespace", func() {
			foundMonitoringNamespace := &corev1.Namespace{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: monitoringNamespace}, foundMonitoringNamespace)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			expectedLabels := map[string]string{
				"kubernetes.io/metadata.name":        monitoringNamespace,
				"opendatahub.io/generated-namespace": "true",
				"openshift.io/cluster-monitoring":    "true",
				"pod-security.kubernetes.io/enforce": "baseline",
			}
			Expect(foundMonitoringNamespace.Name == monitoringNamespace).Should(BeTrue())
			Expect(foundMonitoringNamespace.Labels).To(Equal(expectedLabels))
		})
		It("Should create default rolebinding", func() {
			foundRoleBinding := &authv1.RoleBinding{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{
					Name:      applicationNamespace,
					Namespace: applicationNamespace,
				}, foundRoleBinding)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			expectedSubjects := []authv1.Subject{
				{
					Kind:      "ServiceAccount",
					Namespace: applicationNamespace,
					Name:      workingNamespace,
				},
			}
			expectedRoleRef := authv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "system:openshift:scc:anyuid",
			}
			Expect(foundRoleBinding.Name).To(Equal(applicationNamespace))
			Expect(foundRoleBinding.Namespace).To(Equal(applicationNamespace))
			Expect(foundRoleBinding.Subjects).To(Equal(expectedSubjects))
			Expect(foundRoleBinding.RoleRef).To(Equal(expectedRoleRef))
		})
		It("Should create default configmap", func() {
			foundConfigMap := &corev1.ConfigMap{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{
					Name:      configmapName,
					Namespace: applicationNamespace,
				}, foundConfigMap)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(foundConfigMap.Name).To(Equal(configmapName))
			Expect(foundConfigMap.Namespace).To(Equal(applicationNamespace))
			expectedConfigmapData := map[string]string{"namespace": applicationNamespace}
			Expect(foundConfigMap.Data).To(Equal(expectedConfigmapData))
		})
		It("Should create default networkpolicy", func() {
			foundNetworkPolicy := &netv1.NetworkPolicy{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{
					Name:      applicationNamespace,
					Namespace: applicationNamespace,
				}, foundNetworkPolicy)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(foundNetworkPolicy.Name).To(Equal(applicationNamespace))
			Expect(foundNetworkPolicy.Namespace).To(Equal(applicationNamespace))
			Expect(foundNetworkPolicy.Spec.PolicyTypes[0]).To(Equal(netv1.PolicyTypeIngress))
		})
		AfterAll(func() {
			cleanupResources()
		})
	})

	Context("Should not update rolebinding if it exists", func() {
		ctx := context.Background()
		It("Should create a rolebinding before creating the dsci instance", func() {
			desiredRoleBinding := &authv1.RoleBinding{
				TypeMeta: metav1.TypeMeta{
					Kind:       "RoleBinding",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      applicationNamespace,
					Namespace: applicationNamespace,
				},

				RoleRef: authv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     "system:openshift:scc:anyuid",
				},
			}
			Expect(k8sClient.Create(ctx, desiredRoleBinding)).Should(Succeed())
			createdRoleBinding := &authv1.RoleBinding{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{
					Name:      applicationNamespace,
					Namespace: applicationNamespace,
				}, createdRoleBinding)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(createdRoleBinding.Name).Should(Equal(applicationNamespace))
		})
		It("Should create an instance of DSCI", func() {
			desiredDsci := &dsci.DSCInitialization{
				TypeMeta: metav1.TypeMeta{
					Kind:       "DSCInitialization",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      applicationName,
					Namespace: workingNamespace,
				},
				Spec: dsci.DSCInitializationSpec{
					ApplicationsNamespace: applicationNamespace,
				},
			}
			Expect(k8sClient.Create(ctx, desiredDsci)).Should(Succeed())
			foundDsci := &dsci.DSCInitialization{}
			Eventually(func() bool {
				k8sClient.Get(ctx, client.ObjectKey{
					Name:      applicationName,
					Namespace: workingNamespace,
				}, foundDsci)
				return foundDsci.Status.Phase == readyPhase
			}, timeout, interval).Should(BeTrue())
			Expect(foundDsci.Name).To(Equal(applicationName))
		})
		It("Should not update rolebinding(check subjects)", func() {
			foundRoleBinding := &authv1.RoleBinding{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{
					Name:      applicationNamespace,
					Namespace: applicationNamespace,
				}, foundRoleBinding)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(foundRoleBinding.Subjects).To(BeNil())
		})
		AfterAll(func() {
			cleanupResources()
		})
	})
	Context("Should not update configmap if it exists", func() {
		ctx := context.Background()
		It("Should create a configmap before creating the dsci instance", func() {
			desiredConfigMap := &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      configmapName,
					Namespace: applicationNamespace,
				},
				Data: map[string]string{"namespace": "existing-data"},
			}
			Expect(k8sClient.Create(ctx, desiredConfigMap)).Should(Succeed())
			createdConfigMap := &corev1.ConfigMap{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{
					Name:      configmapName,
					Namespace: applicationNamespace,
				}, createdConfigMap)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(createdConfigMap.Name).Should(Equal(configmapName))
		})
		It("Should create an instance of DSCI", func() {
			desiredDsci := &dsci.DSCInitialization{
				TypeMeta: metav1.TypeMeta{
					Kind:       "DSCInitialization",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      applicationName,
					Namespace: workingNamespace,
				},
				Spec: dsci.DSCInitializationSpec{
					ApplicationsNamespace: applicationNamespace,
				},
			}
			Expect(k8sClient.Create(ctx, desiredDsci)).Should(Succeed())
			foundDsci := &dsci.DSCInitialization{}
			Eventually(func() bool {
				k8sClient.Get(ctx, client.ObjectKey{
					Name:      applicationName,
					Namespace: workingNamespace,
				}, foundDsci)
				return foundDsci.Status.Phase == readyPhase
			}, timeout, interval).Should(BeTrue())
			Expect(foundDsci.Name).To(Equal(applicationName))
		})
		It("Should not update configmaps(check data)", func() {
			foundConfigMap := &corev1.ConfigMap{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{
					Name:      configmapName,
					Namespace: applicationNamespace,
				}, foundConfigMap)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(foundConfigMap.Data).To(Equal(map[string]string{"namespace": "existing-data"}))
			Expect(foundConfigMap.Data).ToNot(Equal(map[string]string{"namespace": applicationNamespace}))
		})
		AfterAll(func() {
			cleanupResources()
		})
	})

	Context("Should not update namespace if it exists", func() {
		ctx := context.Background()
		anotherNamespace := "test-another-ns"
		It("Should create a namespace before creating the dsci instance", func() {
			desiredNamespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: anotherNamespace,
				},
			}
			Expect(k8sClient.Create(ctx, desiredNamespace)).Should(Succeed())
			createdNamespace := &corev1.Namespace{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: anotherNamespace}, createdNamespace)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(createdNamespace.Name).Should(Equal(anotherNamespace))
		})
		It("Should create an instance of DSCI", func() {
			desiredDsci := &dsci.DSCInitialization{
				TypeMeta: metav1.TypeMeta{
					Kind:       "DSCInitialization",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      applicationName,
					Namespace: workingNamespace,
				},
				Spec: dsci.DSCInitializationSpec{
					ApplicationsNamespace: anotherNamespace,
				},
			}
			Expect(k8sClient.Create(ctx, desiredDsci)).Should(Succeed())
			foundDsci := &dsci.DSCInitialization{}
			Eventually(func() bool {
				k8sClient.Get(ctx, client.ObjectKey{
					Name:      applicationName,
					Namespace: workingNamespace,
				}, foundDsci)
				return foundDsci.Status.Phase == readyPhase
			}, timeout, interval).Should(BeTrue())
			Expect(foundDsci.Name).To(Equal(applicationName))
		})
		It("Should not update the namespace(check labels)", func() {
			foundApplicationNamespace := &corev1.Namespace{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: anotherNamespace}, foundApplicationNamespace)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(foundApplicationNamespace.Name == anotherNamespace).To(BeTrue())
			notExpectedLabels := map[string]string{
				"kubernetes.io/metadata.name":        anotherNamespace,
				"opendatahub.io/generated-namespace": "true",
				"pod-security.kubernetes.io/enforce": "baseline",
			}
			expectedLabels := map[string]string{
				"kubernetes.io/metadata.name": anotherNamespace,
			}
			// expect the default labels "opendatahub.io/generated-namespace", "pod-security.kubernetes.io/enforce" to not be added.
			Expect(notExpectedLabels).ToNot(Equal(foundApplicationNamespace.Labels))
			Expect(expectedLabels).To(Equal(foundApplicationNamespace.Labels))
		})
		AfterAll(func() {
			cleanupResources()
		})
	})
})

// cleanup utility func
func cleanupResources() {
	defaultNamespace := client.InNamespace(workingNamespace)
	appNamespace := client.InNamespace(applicationNamespace)
	Expect(k8sClient.DeleteAllOf(context.TODO(), &dsci.DSCInitialization{}, defaultNamespace)).ToNot(HaveOccurred())
	Expect(k8sClient.DeleteAllOf(context.TODO(), &netv1.NetworkPolicy{}, appNamespace)).ToNot(HaveOccurred())
	Expect(k8sClient.DeleteAllOf(context.TODO(), &corev1.ConfigMap{}, appNamespace)).ToNot(HaveOccurred())
	Expect(k8sClient.DeleteAllOf(context.TODO(), &authv1.RoleBinding{}, appNamespace)).ToNot(HaveOccurred())
}
