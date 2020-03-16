/*


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

package v1

import (
	"fmt"
	kbatch "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"time"
)

const (
	ScheduledTimeAnnotation = "my.domain/schduledTime"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ATSpec defines the desired state of AT
type ATSpec struct {
	// JobTemplate that is run by AT
	JobTemplate batchv1beta1.JobTemplateSpec `json:"jobTemplate"`
	// Time which we run this job
	Schedule *metav1.Time `json:"schedule"`
}

type ATScheduleStatus string

const (
	Succeeded ATScheduleStatus = "Succeeded"
	Stale     ATScheduleStatus = "Stale"  // TODO
	Failed    ATScheduleStatus = "Failed" // TODO
	Running   ATScheduleStatus = "Running"
	Pending   ATScheduleStatus = "Pending" // TODO
)

// ATStatus defines the observed state of AT
type ATStatus struct {
	// Status of AT
	// +optional
	LastScheduleTime *metav1.Time `json:"lastScheduleTime,omitempty"`
	// AT Status
	// +optional
	ATScheduleStatus ATScheduleStatus `json:"status,omitempty"`
}

// Update attribute considering childJob status
func (ats *ATStatus) UpdateFromChildJob(childJob kbatch.Job) error {
	// Set LastScheduleTime
	timeRaw := childJob.Annotations[ScheduledTimeAnnotation]
	if len(timeRaw) == 0 {
		return fmt.Errorf("Failed to get %s from childJob", ScheduledTimeAnnotation)
	}

	timeParsed, err := time.Parse(time.RFC3339, timeRaw)
	if err != nil {
		return fmt.Errorf("Failed to parse time annotation %v", err)
	}
	t := metav1.NewTime(timeParsed)
	ats.LastScheduleTime = &t

	if (childJob.Spec.Completions == nil && childJob.Status.Succeeded == 1) ||
		(*childJob.Spec.Completions == childJob.Status.Succeeded) {
		// we consider child job is succeeded if this condition is met
		ats.ATScheduleStatus = Succeeded
	} else {
		// TODO we need to handle Failed,Pending
		ats.ATScheduleStatus = Running
	}
	return nil
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="status",type=string,JSONPath=`.status.status`
// +kubebuilder:printcolumn:name="lastScheduleTime",type=string,JSONPath=`.status.lastScheduleTime`
// +kubebuilder:printcolumn:name="schdule",type=string,JSONPath=`.spec.schedule`
type AT struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ATSpec   `json:"spec,omitempty"`
	Status ATStatus `json:"status,omitempty"`
}

func (at *AT) ChildJobName() string {
	return fmt.Sprintf("%s-%d", at.Name, at.Spec.Schedule.Unix())
}

func (at *AT) ChildJobNameSpacedName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: at.Namespace,
		Name:      at.ChildJobName(),
	}
}

func (at *AT) ConstructChildJob(scheduleTime time.Time) *kbatch.Job {
	labels := make(map[string]string)
	annotations := make(map[string]string)
	for k, v := range at.Spec.JobTemplate.Annotations {
		annotations[k] = v
	}
	annotations[ScheduledTimeAnnotation] = scheduleTime.Format(time.RFC3339)
	for k, v := range at.Spec.JobTemplate.Labels {
		labels[k] = v
	}
	return &kbatch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      labels,
			Annotations: annotations,
			Name:        at.ChildJobName(),
			Namespace:   at.Namespace,
		},
		Spec: *at.Spec.JobTemplate.Spec.DeepCopy(),
	}
}

// +kubebuilder:object:root=true

// ATList contains a list of AT
type ATList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AT `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AT{}, &ATList{})
}
