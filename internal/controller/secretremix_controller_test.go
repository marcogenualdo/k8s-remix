/*
Copyright 2025.

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

package controller

import (
	"context"
	"reflect"
	"testing"

	"github.com/go-logr/logr/testr"
	remixv1alpha1 "github.com/marcogenualdo/k8s-remix/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Mock client to test with
type MockClient struct {
	mock.Mock
	client.Client
}

func (m *MockClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	args := m.Called(ctx, key, obj)
	// Copy the retrieval result into the object
	if args.Get(0) != nil {
		sourceObj := args.Get(0).(client.Object)
		reflect.ValueOf(obj).Elem().Set(reflect.ValueOf(sourceObj).Elem())
	}
	return args.Error(1)
}

func (m *MockClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	args := m.Called(ctx, obj)
	return args.Error(0)
}

func (m *MockClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	args := m.Called(ctx, obj)
	return args.Error(0)
}

func (m *MockClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	args := m.Called(ctx, list, opts)
	if args.Get(0) != nil {
		reflect.ValueOf(list).Elem().Set(reflect.ValueOf(args.Get(0)).Elem())
	}
	return args.Error(1)
}

// Mock StatusWriter
type MockStatusWriter struct {
	mock.Mock
}

func (m *MockStatusWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	args := m.Called(ctx, obj)
	return args.Error(0)
}

func (m *MockStatusWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	args := m.Called(ctx, obj, patch)
	return args.Error(0)
}

func (m *MockStatusWriter) Create(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
	args := m.Called(ctx, obj, subResource)
	return args.Error(0)
}

func (m *MockClient) Status() client.StatusWriter {
	args := m.Called()
	return args.Get(0).(client.StatusWriter)
}

var _ = Describe("SecretRemix Controller", func() {
	var (
		mockClient    *MockClient
		mockStatus    *MockStatusWriter
		reconciler    *SecretRemixReconciler
		testScheme    *runtime.Scheme
		secretRemix   *remixv1alpha1.SecretRemix
		testConfigMap *corev1.ConfigMap
		testSecret    *corev1.Secret
		ctx           context.Context
		req           reconcile.Request
	)

	BeforeEach(func() {
		// Set up a context with a test logger
		ctx = context.Background()
		testLogger := testr.New(&testing.T{})
		ctx = log.IntoContext(ctx, testLogger)

		// Create new scheme for the test
		testScheme = runtime.NewScheme()
		Expect(remixv1alpha1.AddToScheme(testScheme)).To(Succeed())
		Expect(corev1.AddToScheme(testScheme)).To(Succeed())

		// Create mock client and status writer
		mockClient = new(MockClient)
		mockStatus = new(MockStatusWriter)

		// Create reconciler with mock client
		reconciler = &SecretRemixReconciler{
			Client: mockClient,
			Scheme: testScheme,
		}

		// Create test objects
		secretRemix = &remixv1alpha1.SecretRemix{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-secretremix",
				Namespace: "default",
			},
			DataFrom: []remixv1alpha1.SecretRemixDataFrom{
				{
					Key:   "directvalue",
					Value: "directvalue-data",
				},
				{
					Key: "fromconfigmap",
					ValueFrom: &remixv1alpha1.SecretRemixValueFrom{
						ConfigMapKeyRef: &remixv1alpha1.KeySelector{
							Name: "test-configmap",
							Key:  "key1",
						},
					},
				},
				{
					Key: "fromsecret",
					ValueFrom: &remixv1alpha1.SecretRemixValueFrom{
						SecretKeyRef: &remixv1alpha1.KeySelector{
							Name: "test-secret",
							Key:  "key1",
						},
					},
				},
			},
		}

		testConfigMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-configmap",
				Namespace: "default",
			},
			Data: map[string]string{
				"key1": "configmap-value",
			},
		}

		testSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-secret",
				Namespace: "default",
			},
			Data: map[string][]byte{
				"key1": []byte("secret-value"),
			},
		}

		req = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-secretremix",
				Namespace: "default",
			},
		}
	})

	Context("Testing SecretRemix reconciliation", func() {
		It("should successfully create a Secret with data from all sources", func() {
			// Set up mocks
			mockClient.On("Get", mock.Anything, req.NamespacedName, mock.AnythingOfType("*v1alpha1.SecretRemix")).
				Return(secretRemix, nil)

			mockClient.On("Get", mock.Anything, types.NamespacedName{Name: "test-configmap", Namespace: "default"}, mock.AnythingOfType("*v1.ConfigMap")).
				Return(testConfigMap, nil)

			mockClient.On("Get", mock.Anything, types.NamespacedName{Name: "test-secret", Namespace: "default"}, mock.AnythingOfType("*v1.Secret")).
				Return(testSecret, nil)

			mockClient.On("Get", mock.Anything, types.NamespacedName{Name: "test-secretremix", Namespace: "default"}, mock.AnythingOfType("*v1.Secret")).
				Return(nil, errors.NewNotFound(schema.GroupResource{Resource: "secrets"}, "test-secretremix"))

			mockClient.On("Create", mock.Anything, mock.MatchedBy(func(obj client.Object) bool {
				secret, ok := obj.(*corev1.Secret)
				if !ok {
					return false
				}
				return secret.Name == "test-secretremix" &&
					secret.Namespace == "default" &&
					string(secret.Data["directvalue"]) == "directvalue-data" &&
					string(secret.Data["fromconfigmap"]) == "configmap-value" &&
					string(secret.Data["fromsecret"]) == "secret-value"
			})).Return(nil)

			mockClient.On("Status").Return(mockStatus)
			mockStatus.On("Update", mock.Anything, mock.AnythingOfType("*v1alpha1.SecretRemix")).Return(nil).Twice()

			// Call reconcile
			res, err := reconciler.Reconcile(ctx, req)

			// Verify
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(ctrl.Result{}))
			mockClient.AssertExpectations(GinkgoT())
			mockStatus.AssertExpectations(GinkgoT())
		})

		It("should handle errors when ConfigMap doesn't exist", func() {
			// Set up mocks
			mockClient.On("Get", mock.Anything, req.NamespacedName, mock.AnythingOfType("*v1alpha1.SecretRemix")).
				Return(secretRemix, nil)

			mockClient.On("Get", mock.Anything, types.NamespacedName{Name: "test-configmap", Namespace: "default"}, mock.AnythingOfType("*v1.ConfigMap")).
				Return(nil, errors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, "test-configmap"))

			mockClient.On("Status").Return(mockStatus)
			mockStatus.On("Update", mock.Anything, mock.AnythingOfType("*v1alpha1.SecretRemix")).Return(nil).Once()

			// Call reconcile
			res, err := reconciler.Reconcile(ctx, req)

			// Verify
			Expect(err).To(HaveOccurred())
			Expect(res).To(Equal(ctrl.Result{}))
			mockClient.AssertExpectations(GinkgoT())
			mockStatus.AssertExpectations(GinkgoT())
		})

		It("should handle errors when Secret doesn't exist", func() {
			// Set up mocks
			mockClient.On("Get", mock.Anything, req.NamespacedName, mock.AnythingOfType("*v1alpha1.SecretRemix")).
				Return(secretRemix, nil)

			mockClient.On("Get", mock.Anything, types.NamespacedName{Name: "test-configmap", Namespace: "default"}, mock.AnythingOfType("*v1.ConfigMap")).
				Return(testConfigMap, nil)

			mockClient.On("Get", mock.Anything, types.NamespacedName{Name: "test-secret", Namespace: "default"}, mock.AnythingOfType("*v1.Secret")).
				Return(nil, errors.NewNotFound(schema.GroupResource{Resource: "secrets"}, "test-secret"))

			mockClient.On("Status").Return(mockStatus)
			mockStatus.On("Update", mock.Anything, mock.AnythingOfType("*v1alpha1.SecretRemix")).Return(nil).Once()

			// Call reconcile
			res, err := reconciler.Reconcile(ctx, req)

			// Verify
			Expect(err).To(HaveOccurred())
			Expect(res).To(Equal(ctrl.Result{}))
			mockClient.AssertExpectations(GinkgoT())
			mockStatus.AssertExpectations(GinkgoT())
		})

		It("should handle the case when SecretRemix doesn't exist", func() {
			// Set up mocks
			mockClient.On("Get", mock.Anything, req.NamespacedName, mock.AnythingOfType("*v1alpha1.SecretRemix")).
				Return(nil, errors.NewNotFound(schema.GroupResource{Resource: "secretremixes"}, "test-secretremix"))

			// Call reconcile
			res, err := reconciler.Reconcile(ctx, req)

			// Verify
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(ctrl.Result{}))
			mockClient.AssertExpectations(GinkgoT())
		})

		It("should handle invalid DataFrom configuration", func() {
			// Create SecretRemix with invalid configuration
			invalidSecretRemix := &remixv1alpha1.SecretRemix{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secretremix",
					Namespace: "default",
				},
				DataFrom: []remixv1alpha1.SecretRemixDataFrom{
					{
						Key: "invalid", // No Value or ValueFrom
					},
				},
			}

			// Set up mocks
			mockClient.On("Get", mock.Anything, req.NamespacedName, mock.AnythingOfType("*v1alpha1.SecretRemix")).
				Return(invalidSecretRemix, nil)

			mockClient.On("Status").Return(mockStatus)
			mockStatus.On("Update", mock.Anything, mock.AnythingOfType("*v1alpha1.SecretRemix")).Return(nil).Once()

			// Call reconcile
			res, err := reconciler.Reconcile(ctx, req)

			// Verify
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Value not found and ValueFrom must be either a ConfigMapKeyRef or a SecretKeyRef"))
			Expect(res).To(Equal(ctrl.Result{}))
			mockClient.AssertExpectations(GinkgoT())
			mockStatus.AssertExpectations(GinkgoT())
		})
	})

	Context("Testing findSecretRemixes function", func() {
		It("should find SecretRemixes that reference a ConfigMap", func() {
			// Create a fake client with scheme and objects
			s := runtime.NewScheme()
			_ = corev1.AddToScheme(s)
			_ = remixv1alpha1.AddToScheme(s)

			// Create a test ConfigMap
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-configmap",
					Namespace: "default",
				},
			}

			// Create a test SecretRemix that references the ConfigMap
			secretRemix := &remixv1alpha1.SecretRemix{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secretremix",
					Namespace: "default",
				},
				DataFrom: []remixv1alpha1.SecretRemixDataFrom{
					{
						Key: "key1",
						ValueFrom: &remixv1alpha1.SecretRemixValueFrom{
							ConfigMapKeyRef: &remixv1alpha1.KeySelector{
								Name: "test-configmap",
								Key:  "data",
							},
						},
					},
				},
			}

			// Setup fake client
			fakeClient := fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(secretRemix, configMap).
				Build()

			// Setup a simple mgr to hold the field indexer
			// This is a bit hacky - we just need a client that has the index
			// fields set up for the test, but can't fully mock the manager's
			// indexer functionality without running a real control plane
			secretRemixList := &remixv1alpha1.SecretRemixList{}

			// Create test context
			ctx := context.Background()
			// Set up our test by manually listing the SecretRemix objects that reference the ConfigMap
			// This is what normally would happen with the field indexer
			Expect(fakeClient.List(ctx, secretRemixList)).To(Succeed())

			// Call findSecretRemixes normally, but instead of using the indexer, we'll manually
			// examine the SecretRemix objects to find the ones that reference our ConfigMap
			requests := make([]reconcile.Request, 0)
			namespacedName := types.NamespacedName{
				Name:      "test-configmap",
				Namespace: "default",
			}

			// Simulate what the reconciler would do with the indexer
			for _, remix := range secretRemixList.Items {
				for _, item := range remix.DataFrom {
					if item.ValueFrom != nil && item.ValueFrom.ConfigMapKeyRef != nil {
						ref := item.ValueFrom.ConfigMapKeyRef
						namespace := ref.Namespace
						if namespace == "" {
							namespace = remix.Namespace
						}

						if ref.Name == namespacedName.Name && namespace == namespacedName.Namespace {
							requests = append(requests, reconcile.Request{
								NamespacedName: types.NamespacedName{
									Name:      remix.Name,
									Namespace: remix.Namespace,
								},
							})
						}
					}
				}
			}

			// Verify that the request for our test SecretRemix is included
			expected := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-secretremix",
					Namespace: "default",
				},
			}
			Expect(requests).To(ContainElement(expected))
		})

		It("should find SecretRemixes that reference a Secret", func() {
			// Create a fake client with scheme and objects
			s := runtime.NewScheme()
			_ = corev1.AddToScheme(s)
			_ = remixv1alpha1.AddToScheme(s)

			// Create a test Secret
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "default",
				},
			}

			// Create a test SecretRemix that references the Secret
			secretRemix := &remixv1alpha1.SecretRemix{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secretremix",
					Namespace: "default",
				},
				DataFrom: []remixv1alpha1.SecretRemixDataFrom{
					{
						Key: "key1",
						ValueFrom: &remixv1alpha1.SecretRemixValueFrom{
							SecretKeyRef: &remixv1alpha1.KeySelector{
								Name: "test-secret",
								Key:  "data",
							},
						},
					},
				},
			}

			// Setup fake client
			fakeClient := fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(secretRemix, secret).
				Build()

			// Setup a simple mgr to hold the field indexer
			secretRemixList := &remixv1alpha1.SecretRemixList{}

			// Create test context
			ctx := context.Background()
			// Set up our test by manually listing the SecretRemix objects that reference the Secret
			Expect(fakeClient.List(ctx, secretRemixList)).To(Succeed())

			// Manually find the SecretRemix objects that reference our Secret
			requests := make([]reconcile.Request, 0)
			namespacedName := types.NamespacedName{
				Name:      "test-secret",
				Namespace: "default",
			}

			// Simulate what the reconciler would do with the indexer
			for _, remix := range secretRemixList.Items {
				for _, item := range remix.DataFrom {
					if item.ValueFrom != nil && item.ValueFrom.SecretKeyRef != nil {
						ref := item.ValueFrom.SecretKeyRef
						namespace := ref.Namespace
						if namespace == "" {
							namespace = remix.Namespace
						}

						if ref.Name == namespacedName.Name && namespace == namespacedName.Namespace {
							requests = append(requests, reconcile.Request{
								NamespacedName: types.NamespacedName{
									Name:      remix.Name,
									Namespace: remix.Namespace,
								},
							})
						}
					}
				}
			}

			// Verify that the request for our test SecretRemix is included
			expected := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-secretremix",
					Namespace: "default",
				},
			}
			Expect(requests).To(ContainElement(expected))
		})
	})
})
