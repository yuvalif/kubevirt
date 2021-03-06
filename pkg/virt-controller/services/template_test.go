/*
 * This file is part of the KubeVirt project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2017 Red Hat, Inc.
 *
 */

package services_test

import (
	"time"

	. "kubevirt.io/kubevirt/pkg/virt-controller/services"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	kubev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"

	"kubevirt.io/kubevirt/pkg/api/v1"
	"kubevirt.io/kubevirt/pkg/log"
)

var _ = Describe("Template", func() {

	log.Log.SetIOWriter(GinkgoWriter)
	configCache := cache.NewIndexer(cache.DeletionHandlingMetaNamespaceKeyFunc, nil)
	svc := NewTemplateService("kubevirt/virt-launcher", "/var/run/kubevirt", "pull-secret-1", configCache)

	Describe("Rendering", func() {
		Context("launch template with correct parameters", func() {
			It("should work", func() {

				pod, err := svc.RenderLaunchManifest(&v1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: "testvm", Namespace: "testns", UID: "1234"}, Spec: v1.VirtualMachineSpec{Domain: v1.DomainSpec{}}})
				Expect(err).ToNot(HaveOccurred())

				Expect(pod.Spec.Containers[0].Image).To(Equal("kubevirt/virt-launcher"))
				Expect(pod.ObjectMeta.Labels).To(Equal(map[string]string{
					v1.AppLabel:    "virt-launcher",
					v1.DomainLabel: "testvm",
				}))
				Expect(pod.ObjectMeta.Annotations).To(Equal(map[string]string{
					v1.CreatedByAnnotation: "1234",
					v1.OwnedByAnnotation:   "virt-controller",
				}))
				Expect(pod.ObjectMeta.GenerateName).To(Equal("virt-launcher-testvm-"))
				Expect(pod.Spec.NodeSelector).To(Equal(map[string]string{
					v1.NodeSchedulable: "true",
				}))
				Expect(pod.Spec.Containers[0].Command).To(Equal([]string{"/entrypoint.sh",
					"--qemu-timeout", "5m",
					"--name", "testvm",
					"--namespace", "testns",
					"--kubevirt-share-dir", "/var/run/kubevirt",
					"--readiness-file", "/tmp/healthy",
					"--grace-period-seconds", "45"}))
				Expect(*pod.Spec.TerminationGracePeriodSeconds).To(Equal(int64(60)))
				By("setting the right hostname")
				Expect(pod.Spec.Hostname).To(Equal("testvm"))
				Expect(pod.Spec.Subdomain).To(BeEmpty())
			})
		})
		Context("with node selectors", func() {
			It("should add node selectors to template", func() {

				nodeSelector := map[string]string{
					"kubernetes.io/hostname": "master",
					v1.NodeSchedulable:       "true",
				}
				vm := v1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: "testvm", Namespace: "default", UID: "1234"}, Spec: v1.VirtualMachineSpec{NodeSelector: nodeSelector, Domain: v1.DomainSpec{}}}

				pod, err := svc.RenderLaunchManifest(&vm)
				Expect(err).ToNot(HaveOccurred())

				Expect(pod.Spec.Containers[0].Image).To(Equal("kubevirt/virt-launcher"))
				Expect(pod.ObjectMeta.Labels).To(Equal(map[string]string{
					v1.AppLabel:    "virt-launcher",
					v1.DomainLabel: "testvm",
				}))
				Expect(pod.ObjectMeta.GenerateName).To(Equal("virt-launcher-testvm-"))
				Expect(pod.Spec.NodeSelector).To(Equal(map[string]string{
					"kubernetes.io/hostname": "master",
					v1.NodeSchedulable:       "true",
				}))
				Expect(pod.Spec.Containers[0].Command).To(Equal([]string{"/entrypoint.sh",
					"--qemu-timeout", "5m",
					"--name", "testvm",
					"--namespace", "default",
					"--kubevirt-share-dir", "/var/run/kubevirt",
					"--readiness-file", "/tmp/healthy",
					"--grace-period-seconds", "45"}))
				Expect(pod.Spec.Volumes[0].HostPath.Path).To(Equal("/var/run/kubevirt"))
				Expect(pod.Spec.Containers[0].VolumeMounts[0].MountPath).To(Equal("/var/run/kubevirt"))
				Expect(*pod.Spec.TerminationGracePeriodSeconds).To(Equal(int64(60)))
			})

			It("should add node affinity to pod", func() {
				nodeAffinity := kubev1.NodeAffinity{}
				vm := v1.VirtualMachine{
					ObjectMeta: metav1.ObjectMeta{Name: "testvm", Namespace: "default", UID: "1234"},
					Spec: v1.VirtualMachineSpec{
						Affinity: &v1.Affinity{NodeAffinity: &nodeAffinity},
						Domain:   v1.DomainSpec{},
					},
				}
				pod, err := svc.RenderLaunchManifest(&vm)
				Expect(err).ToNot(HaveOccurred())

				Expect(pod.Spec.Affinity).To(BeEquivalentTo(&kubev1.Affinity{NodeAffinity: &nodeAffinity}))
			})

			It("should use the hostname and subdomain if specified on the vm", func() {
				vm := v1.VirtualMachine{
					ObjectMeta: metav1.ObjectMeta{Name: "testvm",
						Namespace: "default",
						UID:       "1234",
					},
					Spec: v1.VirtualMachineSpec{
						Domain:    v1.DomainSpec{},
						Hostname:  "myhost",
						Subdomain: "mydomain",
					},
				}
				pod, err := svc.RenderLaunchManifest(&vm)
				Expect(err).ToNot(HaveOccurred())
				Expect(pod.Spec.Hostname).To(Equal(vm.Spec.Hostname))
				Expect(pod.Spec.Subdomain).To(Equal(vm.Spec.Subdomain))
			})

			It("should add vm labels to pod", func() {
				vm := v1.VirtualMachine{
					ObjectMeta: metav1.ObjectMeta{Name: "testvm",
						Namespace: "default",
						UID:       "1234",
						Labels: map[string]string{
							"key1": "val1",
							"key2": "val2",
						},
					},
					Spec: v1.VirtualMachineSpec{
						Domain: v1.DomainSpec{},
					},
				}
				pod, err := svc.RenderLaunchManifest(&vm)
				Expect(err).ToNot(HaveOccurred())

				Expect(pod.Labels).To(Equal(
					map[string]string{
						"key1":         "val1",
						"key2":         "val2",
						v1.AppLabel:    "virt-launcher",
						v1.DomainLabel: "testvm",
					},
				))
			})

			It("should not add empty node affinity to pod", func() {
				vm := v1.VirtualMachine{
					ObjectMeta: metav1.ObjectMeta{Name: "testvm", Namespace: "default", UID: "1234"},
					Spec: v1.VirtualMachineSpec{
						Domain: v1.DomainSpec{},
					},
				}
				pod, err := svc.RenderLaunchManifest(&vm)
				Expect(err).ToNot(HaveOccurred())

				Expect(pod.Spec.Affinity).To(BeNil())
			})
		})
		Context("with cpu and memory constraints", func() {
			It("should add cpu and memory constraints to a template", func() {

				vm := v1.VirtualMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testvm",
						Namespace: "default",
						UID:       "1234",
					},
					Spec: v1.VirtualMachineSpec{
						Domain: v1.DomainSpec{
							Resources: v1.ResourceRequirements{
								Requests: kubev1.ResourceList{
									kubev1.ResourceCPU:    resource.MustParse("1m"),
									kubev1.ResourceMemory: resource.MustParse("1G"),
								},
								Limits: kubev1.ResourceList{
									kubev1.ResourceCPU:    resource.MustParse("2m"),
									kubev1.ResourceMemory: resource.MustParse("2G"),
								},
							},
						},
					},
				}

				pod, err := svc.RenderLaunchManifest(&vm)
				Expect(err).ToNot(HaveOccurred())

				Expect(pod.Spec.Containers[0].Resources.Requests.Cpu().String()).To(Equal("1m"))
				Expect(pod.Spec.Containers[0].Resources.Limits.Cpu().String()).To(Equal("2m"))
				Expect(pod.Spec.Containers[0].Resources.Requests.Memory().String()).To(Equal("1099507557"))
				Expect(pod.Spec.Containers[0].Resources.Limits.Memory().String()).To(Equal("2099507557"))
			})
			It("should not add unset resources", func() {

				vm := v1.VirtualMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testvm",
						Namespace: "default",
						UID:       "1234",
					},
					Spec: v1.VirtualMachineSpec{
						Domain: v1.DomainSpec{
							CPU: &v1.CPU{Cores: 3},
							Resources: v1.ResourceRequirements{
								Requests: kubev1.ResourceList{
									kubev1.ResourceCPU:    resource.MustParse("1m"),
									kubev1.ResourceMemory: resource.MustParse("64M"),
								},
							},
						},
					},
				}

				pod, err := svc.RenderLaunchManifest(&vm)
				Expect(err).ToNot(HaveOccurred())

				Expect(vm.Spec.Domain.Resources.Requests.Memory().String()).To(Equal("64M"))
				Expect(pod.Spec.Containers[0].Resources.Requests.Cpu().String()).To(Equal("1m"))
				Expect(pod.Spec.Containers[0].Resources.Requests.Memory().ToDec().ScaledValue(resource.Mega)).To(Equal(int64(179)))
				Expect(pod.Spec.Containers[0].Resources.Limits).To(BeNil())
			})
		})

		Context("with pvc source", func() {
			It("should add pvc to template", func() {
				volumes := []v1.Volume{
					{
						Name: "pvc-volume",
						VolumeSource: v1.VolumeSource{
							PersistentVolumeClaim: &kubev1.PersistentVolumeClaimVolumeSource{ClaimName: "nfs-pvc"},
						},
					},
				}
				vm := v1.VirtualMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "testvm", Namespace: "default", UID: "1234",
					},
					Spec: v1.VirtualMachineSpec{Volumes: volumes, Domain: v1.DomainSpec{}},
				}

				pod, err := svc.RenderLaunchManifest(&vm)
				Expect(err).ToNot(HaveOccurred())

				Expect(pod.Spec.Volumes).ToNot(BeEmpty())
				Expect(len(pod.Spec.Volumes)).To(Equal(3))
				Expect(pod.Spec.Volumes[0].PersistentVolumeClaim).ToNot(BeNil())
				Expect(pod.Spec.Volumes[0].PersistentVolumeClaim.ClaimName).To(Equal("nfs-pvc"))
			})
		})

		Context("with launcher's pull secret", func() {
			It("should contain launcher's secret in pod spec", func() {
				vm := v1.VirtualMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "testvm", Namespace: "default", UID: "1234",
					},
					Spec: v1.VirtualMachineSpec{Domain: v1.DomainSpec{}},
				}

				pod, err := svc.RenderLaunchManifest(&vm)
				Expect(err).ToNot(HaveOccurred())

				Expect(len(pod.Spec.ImagePullSecrets)).To(Equal(1))
				Expect(pod.Spec.ImagePullSecrets[0].Name).To(Equal("pull-secret-1"))
			})

		})

		Context("with RegistryDisk pull secrets", func() {
			volumes := []v1.Volume{
				{
					Name: "registrydisk1",
					VolumeSource: v1.VolumeSource{
						RegistryDisk: &v1.RegistryDiskSource{
							Image:           "my-image-1",
							ImagePullSecret: "pull-secret-2",
						},
					},
				},
				{
					Name: "registrydisk2",
					VolumeSource: v1.VolumeSource{
						RegistryDisk: &v1.RegistryDiskSource{
							Image: "my-image-2",
						},
					},
				},
			}

			vm := v1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testvm", Namespace: "default", UID: "1234",
				},
				Spec: v1.VirtualMachineSpec{Volumes: volumes, Domain: v1.DomainSpec{}},
			}

			It("should add secret to pod spec", func() {
				pod, err := svc.RenderLaunchManifest(&vm)
				Expect(err).ToNot(HaveOccurred())

				Expect(len(pod.Spec.ImagePullSecrets)).To(Equal(2))

				// RegistryDisk secrets come first
				Expect(pod.Spec.ImagePullSecrets[0].Name).To(Equal("pull-secret-2"))
				Expect(pod.Spec.ImagePullSecrets[1].Name).To(Equal("pull-secret-1"))
			})

			It("should deduplicate identical secrets", func() {
				volumes[1].VolumeSource.RegistryDisk.ImagePullSecret = "pull-secret-2"

				pod, err := svc.RenderLaunchManifest(&vm)
				Expect(err).ToNot(HaveOccurred())

				Expect(len(pod.Spec.ImagePullSecrets)).To(Equal(2))

				// RegistryDisk secrets come first
				Expect(pod.Spec.ImagePullSecrets[0].Name).To(Equal("pull-secret-2"))
				Expect(pod.Spec.ImagePullSecrets[1].Name).To(Equal("pull-secret-1"))
			})
		})
	})
	Describe("ConfigMap", func() {
		var cmListWatch *cache.ListWatch
		var cmInformer cache.SharedIndexInformer
		var cmStore cache.Store
		var stopChan chan struct{}

		BeforeEach(func() {
			stopChan = make(chan struct{})
		})

		AfterEach(func() {
			close(stopChan)
		})

		It("Should return false if configmap is not present", func() {
			cmListWatch = MakeFakeConfigMapWatcher([]kubev1.ConfigMap{})
			cmInformer = cache.NewSharedIndexInformer(cmListWatch, &v1.VirtualMachine{}, time.Second, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			cmStore = cmInformer.GetStore()
			go cmInformer.Run(stopChan)
			cache.WaitForCacheSync(stopChan, cmInformer.HasSynced)

			result, err := IsEmulationAllowed(cmStore)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeFalse())
		})

		It("Should return false if configmap doesn't have allowEmulation set", func() {
			cfgMap := kubev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "kube-system",
					Name:      "kubevirt-config",
				},
				Data: map[string]string{},
			}
			cmListWatch = MakeFakeConfigMapWatcher([]kubev1.ConfigMap{cfgMap})
			cmInformer = cache.NewSharedIndexInformer(cmListWatch, &v1.VirtualMachine{}, time.Second, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			cmStore = cmInformer.GetStore()
			go cmInformer.Run(stopChan)
			cache.WaitForCacheSync(stopChan, cmInformer.HasSynced)

			result, err := IsEmulationAllowed(cmStore)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeFalse())
		})

		It("Should return true if allowEmulation = true", func() {
			cfgMap := kubev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "kube-system",
					Name:      "kubevirt-config",
				},
				Data: map[string]string{"debug.allowEmulation": "true"},
			}
			cmListWatch = MakeFakeConfigMapWatcher([]kubev1.ConfigMap{cfgMap})
			cmInformer = cache.NewSharedIndexInformer(cmListWatch, &v1.VirtualMachine{}, time.Second, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			cmStore = cmInformer.GetStore()
			go cmInformer.Run(stopChan)
			cache.WaitForCacheSync(stopChan, cmInformer.HasSynced)

			result, err := IsEmulationAllowed(cmStore)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeTrue())
		})
	})
})

func MakeFakeConfigMapWatcher(configMaps []kubev1.ConfigMap) *cache.ListWatch {
	cmListWatch := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return &kubev1.ConfigMapList{Items: configMaps}, nil
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			fakeWatch := watch.NewFake()
			for _, cfgMap := range configMaps {
				fakeWatch.Add(&cfgMap)
			}
			return watch.NewFake(), nil
		},
	}
	return cmListWatch
}
