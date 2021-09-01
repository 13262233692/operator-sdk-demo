/*
Copyright 2021 cnych.

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
	"github.com/cnych/opdemo/v2/resources"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/util/retry"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appv1beta1 "github.com/cnych/opdemo/v2/api/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AppServiceReconciler reconciles a AppService object
type AppServiceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
//	AppService *appv1beta1.AppService

}

var (
	oldSpecAnnotation = "old/spec"
)

//+kubebuilder:rbac:groups=app.ydzs.io,resources=appservices,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=app.ydzs.io,resources=appservices/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=app.ydzs.io,resources=appservices/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the AppService object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.9.2/pkg/reconcile
func (r *AppServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("appservice", req.NamespacedName)

	//ctx = context.Background()

	//logger := r.Log.WithValues("appservice", req.NamespacedName)
	// your logic here
	//业务逻辑实现
	//获取appservice实例
	AppService := &appv1beta1.AppService{}


	err := r.Get(ctx, req.NamespacedName, AppService)
	if err != nil {
		//
		if client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}
	logger.Info("fetch new appservices objecsts", "AppService", AppService)

	//如果不存在，则创建相关资源
	//如果存在，就判断是否需要更新
	//如果需要更新，则直接更新
	//如果不需要更新，则正常返回
	deploy := &appsv1.Deployment{}
	if err := r.Get(ctx, req.NamespacedName, deploy); err != nil && errors.IsNotFound(err) {
		//1 关联Annotations
		data, _ := json.Marshal(AppService.Spec) //data json字符串
		logger.Info("print old appservice annotations appservices objecsts", "AppService", AppService.Annotations)
		if AppService.Annotations != nil {

			AppService.Annotations[oldSpecAnnotation] = string(data)
		} else {
			AppService.Annotations = map[string]string{oldSpecAnnotation: string(data)}
		}
		logger.Info("print new appservice annotations appservices objecsts", "AppService", AppService.Annotations)

		//fmt.Printf("appservice的值是： %v",appService)

		if err := r.Client.Update(ctx, AppService); err != nil {

			return ctrl.Result{}, err

		}
		//创建管理资源
		//2、创建deployment
		deploy := resources.NewDeploy(AppService)
		if err := r.Client.Create(ctx, deploy); err != nil {
			return ctrl.Result{}, err
		}
		//3、创建service
		service := resources.NewService(AppService)
		if err := r.Create(ctx, service); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}
	oldspec := appv1beta1.AppServiceSpec{}

	logger.Info("print edit appservice annotations appservices objecsts", "AppService", AppService.Annotations)


	if err := json.Unmarshal([]byte(AppService.Annotations[oldSpecAnnotation]), &oldspec); err != nil {
		return ctrl.Result{}, err
	}
	// 当前规范与旧版本不一致，则需要更新
	if !reflect.DeepEqual(AppService.Spec, oldspec) {
		//更新关联资源
		newDeploy := resources.NewDeploy(AppService)
		oldDeploy := &appsv1.Deployment{}
		if err := r.Get(ctx, req.NamespacedName, oldDeploy); err != nil {
			return ctrl.Result{}, err
		}
		oldDeploy.Spec = newDeploy.Spec
		if err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := r.Client.Update(ctx, oldDeploy); err != nil {
				return  err
			}
			return nil
		}); err !=nil{
			return ctrl.Result{}, err
		}


		newService := resources.NewService(AppService)
		oldService := corev1.Service{}
		if err := r.Get(ctx, req.NamespacedName, &oldService); err != nil {
			return ctrl.Result{}, err
		}
		newService.Spec.ClusterIP = oldService.Spec.ClusterIP
		oldService.Spec = newService.Spec
		if err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := r.Client.Update(ctx, &oldService); err != nil {
				return err
			}
			return nil
		});err !=nil{
			return ctrl.Result{},err
		}

		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AppServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appv1beta1.AppService{}).
		Complete(r)
}
