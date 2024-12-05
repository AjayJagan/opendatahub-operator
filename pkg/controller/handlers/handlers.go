package handlers

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func LabelToName(key string) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, a client.Object) []reconcile.Request {
		values := a.GetLabels()
		if len(values) == 0 {
			return []reconcile.Request{}
		}

		name := values[key]
		if name == "" {
			return []reconcile.Request{}
		}

		return []reconcile.Request{{
			NamespacedName: types.NamespacedName{
				Name: name,
			},
		}}
	})
}
func AnnotationToName(key string) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		values := obj.GetAnnotations()
		if len(values) == 0 {
			return []reconcile.Request{}
		}

		name := values[key]
		if name == "" {
			return []reconcile.Request{}
		}

		return []reconcile.Request{{
			NamespacedName: types.NamespacedName{
				Name: name,
			},
		}}
	})
}

func Fn(fn func(ctx context.Context, a client.Object) []reconcile.Request) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(fn)
}

func ToNamed(name string) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, a client.Object) []reconcile.Request {
		return []reconcile.Request{{
			NamespacedName: types.NamespacedName{
				Name: name,
			},
		}}
	})
}