package authorization

import (
	"context"
	"reflect"

	"github.com/opendatahub-io/odh-platform/pkg/label"
	"github.com/pkg/errors"
	"istio.io/api/security/v1beta1"
	istiotypev1beta1 "istio.io/api/type/v1beta1"
	istiosecv1beta1 "istio.io/client-go/pkg/apis/security/v1beta1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
)

func (r *PlatformAuthorizationReconciler) reconcilePeerAuthentication(ctx context.Context, target *unstructured.Unstructured) error {
	desired := createPeerAuthentication(r.authComponent.WorkloadSelector, target)
	found := &istiosecv1beta1.PeerAuthentication{}
	justCreated := false

	err := r.Get(ctx, types.NamespacedName{
		Name:      desired.Name,
		Namespace: desired.Namespace,
	}, found)
	if err != nil {
		if apierrs.IsNotFound(err) {
			err = r.Create(ctx, desired)
			if err != nil && !apierrs.IsAlreadyExists(err) {
				return errors.Wrap(err, "unable to create PeerAuthentication")
			}

			justCreated = true
		} else {
			return errors.Wrap(err, "unable to fetch PeerAuthentication")
		}
	}

	// Reconcile the Istio PeerAuthentication if it has been manually modified
	if !justCreated && !ComparePeerAuthentication(desired, found) {
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := r.Get(ctx, types.NamespacedName{
				Name:      desired.Name,
				Namespace: desired.Namespace,
			}, found); err != nil {
				return errors.Wrapf(err, "failed getting PeerAuthentication %s in namespace %s", desired.Name, desired.Namespace)
			}

			found.Spec = *desired.Spec.DeepCopy()
			found.ObjectMeta.Labels = desired.ObjectMeta.Labels

			return errors.Wrap(r.Update(ctx, found), "failed updating PeerAuthentication")
		}); err != nil {
			return errors.Wrap(err, "unable to reconcile the PeerAuthentication")
		}
	}

	return nil
}

func createPeerAuthentication(workloadSelector map[string]string, target *unstructured.Unstructured) *istiosecv1beta1.PeerAuthentication {
	policy := &istiosecv1beta1.PeerAuthentication{
		ObjectMeta: metav1.ObjectMeta{
			Name:        target.GetName(),
			Namespace:   target.GetNamespace(),
			Labels:      label.ApplyStandard(target.GetLabels()),
			Annotations: map[string]string{},
			OwnerReferences: []metav1.OwnerReference{
				targetToOwnerRef(target),
			},
		},
		Spec: v1beta1.PeerAuthentication{
			Selector: &istiotypev1beta1.WorkloadSelector{
				MatchLabels: workloadSelector,
			},
			Mtls: &v1beta1.PeerAuthentication_MutualTLS{Mode: v1beta1.PeerAuthentication_MutualTLS_PERMISSIVE},
		},
	}

	return policy
}

func ComparePeerAuthentication(peerAuth1, peerAuth2 *istiosecv1beta1.PeerAuthentication) bool {
	// .Spec contains MessageState from protobuf which has pragma.DoNotCopy (empty mutex slice)
	// go vet complains about copying mutex when calling DeepEquals on passed variables, when it tries to access it.
	// DeepCopy-ing solves this problem as it's using proto.Clone underneath. This implementation recreates mutex instead of
	// directly copying.
	// Alternatively we could break DeepEquals calls to individual .Spec exported fields, but that might mean ensuring we
	// always compare all relevant fields when API changes.
	peerSpec1 := peerAuth1.Spec.DeepCopy()
	peerSpec2 := peerAuth2.Spec.DeepCopy()

	return reflect.DeepEqual(peerAuth1.ObjectMeta.Labels, peerAuth2.ObjectMeta.Labels) &&
		reflect.DeepEqual(peerSpec1, peerSpec2)
}
