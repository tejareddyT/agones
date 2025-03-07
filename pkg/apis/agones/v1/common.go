// Copyright 2019 Google LLC All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1

import (
	"fmt"

	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// Block of const Error messages
const (
	ErrContainerRequired        = "Container is required when using multiple containers in the pod template"
	ErrHostPort                 = "HostPort cannot be specified with a Dynamic or Passthrough PortPolicy"
	ErrPortPolicyStatic         = "PortPolicy must be Static"
	ErrContainerPortRequired    = "ContainerPort must be defined for Dynamic and Static PortPolicies"
	ErrContainerPortPassthrough = "ContainerPort cannot be specified with Passthrough PortPolicy"
	ErrContainerNameInvalid     = "Container must be empty or the name of a container in the pod template"
)

// AggregatedPlayerStatus stores total player tracking values
type AggregatedPlayerStatus struct {
	Count    int64 `json:"count"`
	Capacity int64 `json:"capacity"`
}

// crd is an interface to get Name and Kind of CRD
type crd interface {
	GetName() string
	GetObjectKind() schema.ObjectKind
}

// validateName Check NameSize of a CRD
func validateName(c crd) []metav1.StatusCause {
	var causes []metav1.StatusCause
	name := c.GetName()
	kind := c.GetObjectKind().GroupVersionKind().Kind
	// make sure the Name of a Fleet does not oversize the Label size in GSS and GS
	if len(name) > validation.LabelValueMaxLength {
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Field:   "Name",
			Message: fmt.Sprintf("Length of %s '%s' name should be no more than 63 characters.", kind, name),
		})
	}
	return causes
}

// gsSpec is an interface which contains all necessary
// functions to perform common validations against it
type gsSpec interface {
	GetGameServerSpec() *GameServerSpec
}

// validateGSSpec Check GameServerSpec of a CRD
// Used by Fleet and GameServerSet
func validateGSSpec(apiHooks APIHooks, gs gsSpec) []metav1.StatusCause {
	gsSpec := gs.GetGameServerSpec()
	gsSpec.ApplyDefaults()
	causes, _ := gsSpec.Validate(apiHooks, "")

	return causes
}

// validateObjectMeta Check ObjectMeta specification
// Used by Fleet, GameServerSet and GameServer
func validateObjectMeta(objMeta *metav1.ObjectMeta) []metav1.StatusCause {
	var causes []metav1.StatusCause

	errs := metav1validation.ValidateLabels(objMeta.Labels, field.NewPath("labels"))
	if len(errs) != 0 {
		for _, v := range errs {
			causes = append(causes, metav1.StatusCause{
				Type:    metav1.CauseTypeFieldValueInvalid,
				Field:   "labels",
				Message: v.Error(),
			})
		}
	}
	errs = apivalidation.ValidateAnnotations(objMeta.Annotations,
		field.NewPath("annotations"))
	if len(errs) != 0 {
		for _, v := range errs {
			causes = append(causes, metav1.StatusCause{
				Type:    metav1.CauseTypeFieldValueInvalid,
				Field:   "annotations",
				Message: v.Error(),
			})
		}
	}
	return causes
}

// AllocationOverflow specifies what labels and/or annotations to apply on Allocated GameServers
// if the desired number of the underlying `GameServerSet` drops below the number of Allocated GameServers
// attached to it.
type AllocationOverflow struct {
	// Labels to be applied to the `GameServer`
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations to be applied to the `GameServer`
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// Validate validates the label and annotation values
func (ao *AllocationOverflow) Validate() ([]metav1.StatusCause, bool) {
	var causes []metav1.StatusCause
	parentField := "Spec.AllocationOverflow"

	errs := metav1validation.ValidateLabels(ao.Labels, field.NewPath(parentField))
	if len(errs) != 0 {
		for _, v := range errs {
			causes = append(causes, metav1.StatusCause{
				Type:    metav1.CauseTypeFieldValueInvalid,
				Field:   "labels",
				Message: v.Error(),
			})
		}
	}
	errs = apivalidation.ValidateAnnotations(ao.Annotations,
		field.NewPath(parentField))
	if len(errs) != 0 {
		for _, v := range errs {
			causes = append(causes, metav1.StatusCause{
				Type:    metav1.CauseTypeFieldValueInvalid,
				Field:   "annotations",
				Message: v.Error(),
			})
		}
	}

	return causes, len(causes) == 0
}

// CountMatches returns the number of Allocated GameServers that match the labels and annotations, and
// the set of GameServers left over.
func (ao *AllocationOverflow) CountMatches(list []*GameServer) (int32, []*GameServer) {
	count := int32(0)
	var rest []*GameServer
	labelSelector := labels.Set(ao.Labels).AsSelector()
	annotationSelector := labels.Set(ao.Annotations).AsSelector()

	for _, gs := range list {
		if gs.Status.State != GameServerStateAllocated {
			continue
		}
		if !labelSelector.Matches(labels.Set(gs.ObjectMeta.Labels)) {
			rest = append(rest, gs)
			continue
		}
		if !annotationSelector.Matches(labels.Set(gs.ObjectMeta.Annotations)) {
			rest = append(rest, gs)
			continue
		}
		count++
	}

	return count, rest
}

// Apply applies the labels and annotations to the passed in GameServer
func (ao *AllocationOverflow) Apply(gs *GameServer) {
	if ao.Annotations != nil {
		if gs.ObjectMeta.Annotations == nil {
			gs.ObjectMeta.Annotations = map[string]string{}
		}
		for k, v := range ao.Annotations {
			gs.ObjectMeta.Annotations[k] = v
		}
	}
	if ao.Labels != nil {
		if gs.ObjectMeta.Labels == nil {
			gs.ObjectMeta.Labels = map[string]string{}
		}
		for k, v := range ao.Labels {
			gs.ObjectMeta.Labels[k] = v
		}
	}
}
