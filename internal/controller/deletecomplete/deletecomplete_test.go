/*
Copyright 2024 The Crossplane Authors.

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

package deletecomplete

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDone(t *testing.T) {
	cases := map[string]struct {
		annotations map[string]string
		want        bool
	}{
		"no annotations":            {nil, false},
		"unrelated annotations":     {map[string]string{"other": "true"}, false},
		"marker set":                {map[string]string{Annotation: AnnotationValue}, true},
		"marker present wrong case": {map[string]string{Annotation: "True"}, false},
		"marker set to false":       {map[string]string{Annotation: "false"}, false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			o := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "x", Annotations: tc.annotations}}
			if got := Done(o); got != tc.want {
				t.Errorf("Done() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestMarkPersists is the regression test for the reason Mark patches rather
// than assigning in memory: the reconciler only writes back status, so an
// in-memory annotation is discarded with the reconcile and the managed resource
// never terminates.
func TestMarkPersists(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("cannot build scheme: %v", err)
	}

	obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "ns"}}
	kube := fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj).Build()

	if err := kube.Get(context.Background(), client.ObjectKey{Name: "x", Namespace: "ns"}, obj); err != nil {
		t.Fatalf("cannot read seed object: %v", err)
	}

	if err := Mark(context.Background(), kube, obj); err != nil {
		t.Fatalf("Mark failed: %v", err)
	}

	stored := &corev1.ConfigMap{}
	if err := kube.Get(context.Background(), client.ObjectKey{Name: "x", Namespace: "ns"}, stored); err != nil {
		t.Fatalf("cannot read back object: %v", err)
	}

	if got := stored.GetAnnotations()[Annotation]; got != AnnotationValue {
		t.Fatalf("expected the annotation to be persisted, got %q", got)
	}
	if !Done(stored) {
		t.Error("Done() must report true for the persisted object")
	}
}

// TestMarkPreservesExistingAnnotations guards against clobbering annotations
// somebody else owns, such as crossplane.io/external-name.
func TestMarkPreservesExistingAnnotations(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("cannot build scheme: %v", err)
	}

	obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name:        "x",
		Namespace:   "ns",
		Annotations: map[string]string{"crossplane.io/external-name": "keep-me"},
	}}
	kube := fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj).Build()

	if err := kube.Get(context.Background(), client.ObjectKey{Name: "x", Namespace: "ns"}, obj); err != nil {
		t.Fatalf("cannot read seed object: %v", err)
	}

	if err := Mark(context.Background(), kube, obj); err != nil {
		t.Fatalf("Mark failed: %v", err)
	}

	stored := &corev1.ConfigMap{}
	if err := kube.Get(context.Background(), client.ObjectKey{Name: "x", Namespace: "ns"}, stored); err != nil {
		t.Fatalf("cannot read back object: %v", err)
	}

	if got := stored.GetAnnotations()["crossplane.io/external-name"]; got != "keep-me" {
		t.Errorf("existing annotations must be preserved, got %q", got)
	}
	if got := stored.GetAnnotations()[Annotation]; got != AnnotationValue {
		t.Errorf("expected the marker to be set, got %q", got)
	}
}

// TestMarkSurfacesFailure makes sure a failure to persist is reported rather
// than swallowed, so the caller does not treat an unrecorded release as done.
func TestMarkSurfacesFailure(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("cannot build scheme: %v", err)
	}

	// Built without the object, so the patch fails.
	kube := fake.NewClientBuilder().WithScheme(scheme).Build()

	obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "missing", Namespace: "ns"}}

	if err := Mark(context.Background(), kube, obj); err == nil {
		t.Fatal("expected a failure to persist to be reported")
	}
}
