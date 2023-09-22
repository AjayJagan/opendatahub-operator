package dscinitialization

import (
	"context"
	"fmt"

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
	workingNamespace     = "default"
	applicationNamespace = "test-application-ns"
	monitoringNamespace  = "test-monitoring-ns"
	configmapName        = "odh-common-config"
	readyPhase           = "Ready"
)

var _ = Describe("DataScienceCluster initialization", func() {
	Context("Creation of related resources", func() {
		ctx := context.Background()
		applicationName := "default-test"
		BeforeEach(func() {
			desiredDsci := getNewInstance(applicationName)
			Expect(k8sClient.Create(ctx, desiredDsci)).Should(Succeed())
			foundDsci := &dsci.DSCInitialization{}
			Eventually(func() bool {
				k8sClient.Get(ctx, client.ObjectKey{
					Name:      applicationName,
					Namespace: workingNamespace,
				}, foundDsci)
				return foundDsci.Status.Phase == readyPhase
			}, timeout, interval).Should(BeTrue())
		})
		AfterEach(func() {
			cleanupResources()
		})
		It("Should create all the default resources", func() {

			By("Checking default application namespace")
			foundApplicationNamespace := corev1.Namespace{}
			//objectExists("", applicationName, foundApplicationNamespace)
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: applicationNamespace}, foundApplicationNamespace)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			fmt.Print(foundApplicationNamespace)
			Expect(foundApplicationNamespace.Name).To(Equal(applicationNamespace))
			expectedLabels := map[string]string{
				"kubernetes.io/metadata.name":        applicationNamespace,
				"opendatahub.io/generated-namespace": "true",
				"pod-security.kubernetes.io/enforce": "baseline",
			}
			Expect(foundApplicationNamespace.Labels).To(Equal(expectedLabels))

			By("Checking default monitoring namespace")
			foundMonitoringNamespace := &corev1.Namespace{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: monitoringNamespace}, foundMonitoringNamespace)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			expectedLabels = map[string]string{
				"kubernetes.io/metadata.name":        monitoringNamespace,
				"opendatahub.io/generated-namespace": "true",
				"openshift.io/cluster-monitoring":    "true",
				"pod-security.kubernetes.io/enforce": "baseline",
			}
			Expect(foundMonitoringNamespace.Name == monitoringNamespace).Should(BeTrue())
			Expect(foundMonitoringNamespace.Labels).To(Equal(expectedLabels))

			By("Checking default network policy")
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

			By("Checking default configmap")
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

			By("Checking default rolebinding")
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
	})
	Context("Handling existing resources", func() {
		AfterEach(func() {
			cleanupResources()
		})
		It("Should not update rolebinding if it exists", func() {
			applicationName := "rolebinding-test"

			By("Creating a rolebinding before creating the dsci instance")
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

			By("Creating a dsci instance when a rolebinding exists")
			desiredDsci := getNewInstance(applicationName)
			Expect(k8sClient.Create(ctx, desiredDsci)).Should(Succeed())
			foundDsci := &dsci.DSCInitialization{}
			Eventually(func() bool {
				k8sClient.Get(ctx, client.ObjectKey{
					Name:      applicationName,
					Namespace: workingNamespace,
				}, foundDsci)
				return foundDsci.Status.Phase == readyPhase
			}, timeout, interval).Should(BeTrue())

			By("Checking if the rolebinding is not updated")
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

		It("Should not update configmap if it exists", func() {
			applicationName := "configmap-test"

			By("Creating a configmap before creating the dsci instance")
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

			By("Creating a dsci instance when a configmap exists")
			desiredDsci := getNewInstance(applicationName)
			Expect(k8sClient.Create(ctx, desiredDsci)).Should(Succeed())
			foundDsci := &dsci.DSCInitialization{}
			Eventually(func() bool {
				k8sClient.Get(ctx, client.ObjectKey{
					Name:      applicationName,
					Namespace: workingNamespace,
				}, foundDsci)
				return foundDsci.Status.Phase == readyPhase
			}, timeout, interval).Should(BeTrue())

			By("Checking if the configmap is not updated")
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

		It("Should not update namespace if it exists", func() {
			applicationName := "configmap-test"
			anotherNamespace := "test-another-ns"

			By("Creating a namespace before creating the dsci instance")
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

			By("Creating a dsci instance when a namespace exists")
			desiredDsci := getNewInstance(applicationName)
			Expect(k8sClient.Create(ctx, desiredDsci)).Should(Succeed())
			foundDsci := &dsci.DSCInitialization{}
			Eventually(func() bool {
				k8sClient.Get(ctx, client.ObjectKey{
					Name:      applicationName,
					Namespace: workingNamespace,
				}, foundDsci)
				return foundDsci.Status.Phase == readyPhase
			}, timeout, interval).Should(BeTrue())

			By("Checking if the namespace is not updated")
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

func getNewInstance(appName string) *dsci.DSCInitialization {
	return &dsci.DSCInitialization{
		TypeMeta: metav1.TypeMeta{
			Kind:       "DSCInitialization",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      appName,
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
}

func objectExists(ns string, name string, obj client.Object) func() bool {
	var objKey client.ObjectKey
	if ns == "" {
		objKey = client.ObjectKey{
			Name: name,
		}
	} else {
		objKey = client.ObjectKey{
			Name:      name,
			Namespace: ns,
		}
	}
	return func() bool {
		err := k8sClient.Get(ctx, objKey, obj)
		return err == nil
	}

}
