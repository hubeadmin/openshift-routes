/*
Copyright 2022 The cert-manager Authors.

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

	cmapi "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmclient "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	"github.com/go-logr/logr"
	routev1 "github.com/openshift/api/route/v1"
	routev1client "github.com/openshift/client-go/route/clientset/versioned"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/cert-manager/openshift-routes/internal/cmd/app/options"
)

type Route struct {
	routeClient   routev1client.Interface
	certClient    cmclient.Interface
	eventRecorder record.EventRecorder

	log logr.Logger
}

func (r *Route) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("object", req.NamespacedName)
	log.V(5).Info("started reconciling")
	route, err := r.routeClient.RouteV1().Routes(req.Namespace).Get(ctx, req.Name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return reconcile.Result{}, nil
	}
	if err != nil {
		return reconcile.Result{}, err
	}
	log.V(5).Info("retrieved route")
	if metav1.HasAnnotation(route.ObjectMeta, cmapi.IssuerNameAnnotationKey) {
		log.V(5).Info("route has cert-manager annotation, reconciling", cmapi.IssuerNameAnnotationKey, route.Annotations[cmapi.IssuerNameAnnotationKey])
		return r.sync(ctx, req, route.DeepCopy())
	} else if metav1.HasAnnotation(route.ObjectMeta, cmapi.IngressIssuerNameAnnotationKey) {
		log.V(5).Info("route has cert-manager annotation, reconciling", cmapi.IngressIssuerNameAnnotationKey, route.Annotations[cmapi.IngressIssuerNameAnnotationKey])
		return r.sync(ctx, req, route.DeepCopy())
	}
	log.V(5).Info("ignoring route without cert-manager issuer name annotation")
	return reconcile.Result{}, nil
}

func New(base logr.Logger, config *rest.Config, recorder record.EventRecorder) (*Route, error) {
	routeClient, err := routev1client.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	certClient, err := cmclient.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Route{
		routeClient:   routeClient,
		certClient:    certClient,
		log:           base.WithName("route"),
		eventRecorder: recorder,
	}, nil
}

func AddToManager(mgr manager.Manager, opts *options.Options) error {
	controller, err := New(opts.Logr, opts.RestConfig, opts.EventRecorder)
	if err != nil {
		return err
	}
	return builder.
		ControllerManagedBy(mgr).
		For(&routev1.Route{}).
		Owns(&cmapi.CertificateRequest{}).
		Complete(controller)
}
