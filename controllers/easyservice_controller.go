/*
Copyright 2023.

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
	"encoding/json"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appv1 "operator-example/api/v1"
	resources "webapp-operator/resources"
)

// EasyServiceReconciler reconciles a EasyService object
type EasyServiceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=app.kubebuilder.io,resources=easyservices,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=app.kubebuilder.io,resources=easyservices/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=app.kubebuilder.io,resources=easyservices/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the EasyService object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile

func (r *EasyServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// _ = log.FromContext(ctx)
	log := log.FromContext(ctx)

	// TODO(user): your logic here
	// 业务逻辑实现
	// 获取 EasyService 实例
	var easyService appv1.EasyService
	err := r.Get(ctx, req.NamespacedName, &easyService)
	if err != nil {
		// MyApp 被删除的时候，忽略
		if client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	log.Info("fetch easyservice objects", "easyservice", easyService)

	// 如果不存在，则创建关联资源
	// 如果存在，判断是否需要更新
	//   如果需要更新，则直接更新
	//   如果不需要更新，则正常返回
	deploy := &appsv1.Deployment{}
	if err := r.Get(ctx, req.NamespacedName, deploy); err != nil && errors.IsNotFound(err) {
		// 1. 关联 Annotations
		data, _ := json.Marshal(easyService.Spec)
		if easyService.Annotations != nil {
			easyService.Annotations["spec"] = string(data)
		} else {
			easyService.Annotations = map[string]string{"spec": string(data)}
		}
		if err := r.Client.Update(ctx, &easyService); err != nil {
			return ctrl.Result{}, err
		}
		// 创建关联资源
		// 2. 创建 Deployment
		deploy := resources.NewDeploy(&easyService)
		if err := r.Client.Create(ctx, deploy); err != nil {
			return ctrl.Result{}, err
		}
		// 3. 创建 Service
		service := resources.NewService(&easyService)
		if err := r.Create(ctx, service); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}
	oldspec := appv1.EasyServiceSpec{}
	if err := json.Unmarshal([]byte(easyService.Annotations["spec"]), &oldspec); err != nil {
		return ctrl.Result{}, err
	}
	// 当前规范与旧的对象不一致，则需要更新
	if !reflect.DeepEqual(easyService.Spec, oldspec) {
		// 更新关联资源
		newDeploy := resources.NewDeploy(&easyService)
		oldDeploy := &appsv1.Deployment{}
		if err := r.Get(ctx, req.NamespacedName, oldDeploy); err != nil {
			return ctrl.Result{}, err
		}
		oldDeploy.Spec = newDeploy.Spec
		if err := r.Client.Update(ctx, oldDeploy); err != nil {
			return ctrl.Result{}, err
		}

		newService := resources.NewService(&easyService)
		oldService := &corev1.Service{}
		if err := r.Get(ctx, req.NamespacedName, oldService); err != nil {
			return ctrl.Result{}, err
		}
		// 需要指定 ClusterIP 为之前的，不然更新会报错
		newService.Spec.ClusterIP = oldService.Spec.ClusterIP
		oldService.Spec = newService.Spec
		if err := r.Client.Update(ctx, oldService); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}



// SetupWithManager sets up the controller with the Manager.
func (r *EasyServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appv1.EasyService{}).
		Complete(r)
}
