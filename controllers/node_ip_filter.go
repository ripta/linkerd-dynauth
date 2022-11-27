package controllers

import (
	corev1 "k8s.io/api/core/v1"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// nodeIPFilter is an event predicate that skips over any change that did not
// involve changing the IP addresses of a node. This predicate allows us to
// mostly ignore node heartbeats.
type nodeIPFilter struct{}

var _ predicate.Predicate = &nodeIPFilter{}

func (n nodeIPFilter) Create(_ event.CreateEvent) bool {
	// Always process node creation events
	return true
}

func (n nodeIPFilter) Delete(_ event.DeleteEvent) bool {
	// Always process node deletion events
	return true
}

func (n nodeIPFilter) Update(e event.UpdateEvent) bool {
	// Skip if we've seen the change before
	if e.ObjectNew.GetGeneration() == e.ObjectOld.GetGeneration() {
		return false
	}

	newNode, ok := e.ObjectNew.(*corev1.Node)
	if !ok {
		return false
	}

	oldNode, ok := e.ObjectOld.(*corev1.Node)
	if !ok {
		return false
	}

	// Skip over any change that have not caused node IPs to change
	if reflect.DeepEqual(newNode.Status.Addresses, oldNode.Status.Addresses) {
		return false
	}

	return true
}

func (n nodeIPFilter) Generic(_ event.GenericEvent) bool {
	return false
}
