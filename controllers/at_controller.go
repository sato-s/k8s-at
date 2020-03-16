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

package controllers

import (
	"context"

	"fmt"
	"github.com/go-logr/logr"
	kbatch "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"

	batchv1 "github.com/sato-s/k8s-at/api/v1"
)

// ATReconciler reconciles a AT object
type ATReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=batch.my.domain,resources=ats,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch.my.domain,resources=ats/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get

func (r *ATReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("at", req.NamespacedName)
	now := time.Now()

	// Get AT. exit if we can't find this.
	var at batchv1.AT
	if err := r.Get(ctx, req.NamespacedName, &at); err != nil {
		log.Error(err, "unable to fetch AT")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var childJob kbatch.Job
	// Try to find child job
	err := r.Get(ctx, at.ChildJobNameSpacedName(), &childJob)
	if err == nil {
		// We found child job
		log.Info("child job found", "Job", childJob.Name)
		if err := at.Status.UpdateFromChildJob(childJob); err == nil {
			if err := r.Status().Update(ctx, &at); err != nil {
				return ctrl.Result{}, fmt.Errorf("Failed to update at status %v", err)
			} else {
				log.Info("Succesfully update AT info")
				return ctrl.Result{}, nil
			}
		} else {
			return ctrl.Result{}, fmt.Errorf("Failed to update at status %v", err)
		}
	} else if !apierrors.IsNotFound(err) {
		// We have api error other than NotFound
		log.Error(err, "Failed to fetch child job")
		return ctrl.Result{}, err
	}
	// We dont' have child job
	nextRunTime := at.Spec.Schedule.Sub(now)
	if nextRunTime < 0 {
		log.Info("Time to run. create child job.")
		newJob := at.ConstructChildJob(now)
		// Set owner reference to newJob
		if err := ctrl.SetControllerReference(&at, newJob, r.Scheme); err != nil {
			return ctrl.Result{}, fmt.Errorf("Failed to set owener reference to newJob %v", err)
		}
		if err := r.Create(ctx, newJob); err != nil {
			return ctrl.Result{}, fmt.Errorf("Failed to create newJob %v", err)
		} else {
			log.Info("Successfully create newJob", "job", newJob.Name)
			return ctrl.Result{}, nil
		}
	} else {
		log.Info("It's not time to run child job", "now", now, "next run", nextRunTime)
		at.Status.ATScheduleStatus = batchv1.Pending
		if err := r.Status().Update(ctx, &at); err != nil {
			return ctrl.Result{}, fmt.Errorf("Failed to update at status %v", err)
		} else {
			return ctrl.Result{RequeueAfter: nextRunTime}, nil
		}
	}

}

var (
	jobOwnerKey = ".metadata.controller"
	apiGVStr    = batchv1.GroupVersion.String()
)

func (r *ATReconciler) SetupWithManager(mgr ctrl.Manager) error {

	if err := mgr.GetFieldIndexer().IndexField(&kbatch.Job{}, jobOwnerKey, func(rawObj runtime.Object) []string {
		// grab the job object, extract the owner...
		job := rawObj.(*kbatch.Job)
		owner := metav1.GetControllerOf(job)
		if owner == nil {
			return nil
		}
		if owner.APIVersion != apiGVStr || owner.Kind != "AT" {
			return nil
		}
		return []string{owner.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&batchv1.AT{}).
		Owns(&kbatch.Job{}).
		Complete(r)
}
