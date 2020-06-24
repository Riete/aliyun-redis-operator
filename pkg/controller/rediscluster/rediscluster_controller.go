package rediscluster

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/labels"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"

	redisclusterv1alpha1 "github.com/riete/redis-cluster-operator/pkg/apis/middleware/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_rediscluster")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new RedisCluster Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileRedisCluster{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("rediscluster-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource RedisCluster
	err = c.Watch(&source.Kind{Type: &redisclusterv1alpha1.RedisCluster{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner RedisCluster
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &redisclusterv1alpha1.RedisCluster{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileRedisCluster implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileRedisCluster{}

// ReconcileRedisCluster reconciles a RedisCluster object
type ReconcileRedisCluster struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a RedisCluster object and makes changes based on the state read
// and what is in the RedisCluster.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileRedisCluster) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling RedisCluster")

	// Fetch the RedisCluster instance
	instance := &redisclusterv1alpha1.RedisCluster{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Define a new service object
	service := newService(instance)

	// Set RedisCluster instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, service, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check if service already exists
	serviceFound := &corev1.Service{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: service.Name, Namespace: service.Namespace}, serviceFound)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new service", "Service.Namespace", service.Namespace, "Service.Name", service.Name)
		err = r.client.Create(context.TODO(), service)
		if err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}

	// Define a new statefulset object
	statefulset := newStatefulSet(instance)

	// Set RedisCluster instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, statefulset, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check if statefulset already exists
	statefulsetFound := &appsv1.StatefulSet{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: statefulset.Name, Namespace: statefulset.Namespace}, statefulsetFound)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new statefulset", "StatefulSet.Namespace", statefulset.Namespace, "StatefulSet.Name", statefulset.Name)
		err = r.client.Create(context.TODO(), statefulset)
		if err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}

	if *statefulsetFound.Spec.Replicas != statefulsetFound.Status.ReadyReplicas {
		reqLogger.Info("Wait all pod ready", "StatefulSet.Namespace",
			statefulsetFound.Namespace, "StatefulSet.Name", statefulsetFound.Name, "total", *statefulsetFound.Spec.Replicas,
			"current", statefulsetFound.Status.ReadyReplicas)
		return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 15}, nil
	}

	// init cluster
	// Get statefulset pod ips
	podList := &corev1.PodList{}
	labelSelector := labels.SelectorFromSet(map[string]string{"app": instance.Name})
	listOps := &client.ListOptions{Namespace: instance.Namespace, LabelSelector: labelSelector}
	err = r.client.List(context.TODO(), podList, listOps)
	if err != nil {
		reqLogger.Error(err, "Failed to list pods", "StatefulSet.Namespace", instance.Namespace, "StatefulSet.Name", instance.Name)
		return reconcile.Result{}, err
	}
	var podIps []string
	var podAlias []string
	port := getPortFromEnv(instance)
	for _, pod := range podList.Items {
		// ensure other pod with same label excluded
		ownerReference := pod.ObjectMeta.OwnerReferences[0]
		if ownerReference.Name == instance.Name && ownerReference.Kind == "StatefulSet" {
			podIps = append(podIps, fmt.Sprintf("%s:%d", pod.Status.PodIP, port))
			podAlias = append(podAlias, pod.Name)
		}
	}

	// Define a new job object
	job := newClusterInitJob(instance, podIps)

	// Set RedisCluster instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, job, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check if statefulset already exists
	jobFound := &batchv1.Job{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: job.Name, Namespace: job.Namespace}, jobFound)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new job", "Job.Namespace", job.Namespace, "Job.Name", job.Name)
		err = r.client.Create(context.TODO(), job)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	// Define a redis_exporter deployment
	if instance.Spec.Metrics {
		exporter := newExporterDeployment(instance, podAlias)
		if err := controllerutil.SetControllerReference(instance, exporter, r.scheme); err != nil {
			return reconcile.Result{}, err
		}
		exporterFound := &appsv1.Deployment{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: exporter.Name, Namespace: exporter.Namespace}, exporterFound)
		if err != nil && errors.IsNotFound(err) {
			reqLogger.Info("Creating a new deployment", "Deployment.Namespace", exporter.Namespace, "Deployment.Name", exporter.Name)
			err = r.client.Create(context.TODO(), exporter)
			if err != nil {
				return reconcile.Result{}, err
			}
		}
	}
	// cluster already exists - don't requeue
	reqLogger.Info("Skip reconcile: cluster already exists", "RedisCluster.Namespace", instance.Namespace, "RedisCluster.Name", instance.Name)
	return reconcile.Result{}, nil
}

func getEnvValue(rc *redisclusterv1alpha1.RedisCluster, name string) string {
	for _, v := range rc.Spec.Env {
		if v.Name == name {
			return v.Value
		}
	}
	return ""
}

func getPortFromEnv(rc *redisclusterv1alpha1.RedisCluster) int {
	value := getEnvValue(rc, "REDIS_PORT")
	if value == "" {
		return 6379
	}
	port, _ := strconv.Atoi(value)
	return port
}

func newExporterDeployment(rc *redisclusterv1alpha1.RedisCluster, podAlias []string) *appsv1.Deployment {
	var replicas int32 = 1
	port := getPortFromEnv(rc)
	cpuRequest, _ := resource.ParseQuantity("10m")
	cpuLimit, _ := resource.ParseQuantity("50m")
	memoryRequest, _ := resource.ParseQuantity("20Mi")
	memoryLimit, _ := resource.ParseQuantity("100Mi")
	name := fmt.Sprintf("%s-exporter", rc.Name)
	label := map[string]string{"app": name}
	var podDns []string
	for _, v := range podAlias {
		podDns = append(podDns, fmt.Sprintf("%s.%s:%d", v, rc.Name, port))
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: rc.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: label,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: label,
					Annotations: map[string]string{
						"prometheus.io/path":   "/metrics",
						"prometheus.io/port":   strconv.Itoa(int(redisclusterv1alpha1.RedisExporterPort)),
						"prometheus.io/scrape": "true",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image: redisclusterv1alpha1.RedisExporterImage,
						Name:  name,
						Ports: []corev1.ContainerPort{{
							ContainerPort: redisclusterv1alpha1.RedisExporterPort,
							Name:          "redis-exporter",
							Protocol:      corev1.ProtocolTCP,
						}},
						LivenessProbe: &corev1.Probe{
							FailureThreshold:    3,
							InitialDelaySeconds: 10,
							PeriodSeconds:       10,
							SuccessThreshold:    1,
							TimeoutSeconds:      3,
							Handler: corev1.Handler{
								TCPSocket: &corev1.TCPSocketAction{
									Port: intstr.FromInt(int(redisclusterv1alpha1.RedisExporterPort)),
								},
							},
						},
						ReadinessProbe: &corev1.Probe{
							FailureThreshold:    3,
							InitialDelaySeconds: 10,
							PeriodSeconds:       10,
							SuccessThreshold:    1,
							TimeoutSeconds:      3,
							Handler: corev1.Handler{
								TCPSocket: &corev1.TCPSocketAction{
									Port: intstr.FromInt(int(redisclusterv1alpha1.RedisExporterPort)),
								},
							},
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    cpuRequest,
								corev1.ResourceMemory: memoryRequest,
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    cpuLimit,
								corev1.ResourceMemory: memoryLimit,
							},
						},
					}},
				},
			},
		},
	}
	password := getEnvValue(rc, "REDIS_PASSWORD")
	if password != "" {
		replicas := (rc.Spec.Replicas + 1) * 3
		var passwords []string
		for i := 0; i < int(replicas); i++ {
			passwords = append(passwords, password)
		}
		deployment.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
			{Name: "REDIS_ALIAS", Value: strings.Join(podAlias, ",")},
			{Name: "REDIS_ADDR", Value: strings.Join(podDns, ",")},
			{Name: "REDIS_PASSWORD", Value: strings.Join(passwords, ",")},
		}
	} else {
		deployment.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
			{Name: "REDIS_ALIAS", Value: strings.Join(podAlias, ",")},
			{Name: "REDIS_ADDR", Value: strings.Join(podDns, ",")},
		}
	}

	return deployment
}

func newClusterInitJob(rc *redisclusterv1alpha1.RedisCluster, podIps []string) *batchv1.Job {
	var retry int32 = 0
	args := []string{"create", fmt.Sprintf("--replicas %d", rc.Spec.Replicas)}
	args = append(args, podIps...)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rc.Name,
			Namespace: rc.Namespace,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &retry,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:            "redis-cluster-init",
							Image:           redisclusterv1alpha1.RedisTribImage,
							ImagePullPolicy: corev1.PullAlways,
							Args:            args,
						},
					},
				},
			},
		},
	}
	password := getEnvValue(rc, "REDIS_PASSWORD")
	if password != "" {
		job.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
			{Name: "REDIS_PASSWORD", Value: password},
			{Name: "ACCEPT", Value: "yes"},
		}
	} else {
		job.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
			{Name: "ACCEPT", Value: "yes"},
		}
	}

	return job
}

func newService(rc *redisclusterv1alpha1.RedisCluster) *corev1.Service {
	label := map[string]string{"app": rc.Name}
	port := getPortFromEnv(rc)
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rc.Name,
			Namespace: rc.Namespace,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
			Selector:  label,
			Type:      corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{{
				Name:       "redis",
				Protocol:   corev1.ProtocolTCP,
				Port:       int32(port),
				TargetPort: intstr.FromInt(port),
			}},
		},
	}
}

func newStatefulSet(rc *redisclusterv1alpha1.RedisCluster) *appsv1.StatefulSet {
	privileged := true
	port := getPortFromEnv(rc)
	label := map[string]string{"app": rc.Name}
	replicas := (rc.Spec.Replicas + 1) * 3
	storageSize, _ := resource.ParseQuantity(rc.Spec.StorageSize)
	redisImage := redisclusterv1alpha1.RedisClusterImage3
	if rc.Spec.MajorVersion == 4 {
		redisImage = redisclusterv1alpha1.RedisClusterImage4
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rc.Name,
			Namespace: rc.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &replicas,
			ServiceName: rc.Name,
			Selector: &metav1.LabelSelector{
				MatchLabels: label,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: label,
				},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{{
						Image: redisclusterv1alpha1.BusyBoxImage,
						Name:  "set-net-core-somaxconn",
						SecurityContext: &corev1.SecurityContext{
							Privileged: &privileged,
						},
						Command: []string{"sh", "-c", "echo 511 > /proc/sys/net/core/somaxconn"},
					}},
					Containers: []corev1.Container{{
						Image:           redisImage,
						ImagePullPolicy: corev1.PullAlways,
						Name:            rc.Name,
						Ports: []corev1.ContainerPort{{
							ContainerPort: int32(port),
							Name:          "redis",
							Protocol:      corev1.ProtocolTCP,
						}},
						LivenessProbe: &corev1.Probe{
							FailureThreshold:    10,
							InitialDelaySeconds: 6,
							PeriodSeconds:       6,
							SuccessThreshold:    1,
							TimeoutSeconds:      3,
							Handler: corev1.Handler{
								TCPSocket: &corev1.TCPSocketAction{
									Port: intstr.FromInt(port),
								},
							},
						},
						ReadinessProbe: &corev1.Probe{
							FailureThreshold:    10,
							InitialDelaySeconds: 6,
							PeriodSeconds:       6,
							SuccessThreshold:    1,
							TimeoutSeconds:      3,
							Handler: corev1.Handler{
								TCPSocket: &corev1.TCPSocketAction{
									Port: intstr.FromInt(port),
								},
							},
						},
						Resources: rc.Spec.Resources,
						Env: append(rc.Spec.Env, corev1.EnvVar{Name: "POD_IP", ValueFrom: &corev1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{
								APIVersion: "v1",
								FieldPath:  "status.podIP",
							},
						}}),
					}},
				},
			},
		},
	}

	if rc.Spec.StorageClass != "" {
		sts.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
			{Name: "data", MountPath: "/data"},
		}
		sts.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{{
			ObjectMeta: metav1.ObjectMeta{
				Name: "data",
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: storageSize,
					},
				},
				StorageClassName: &rc.Spec.StorageClass,
			},
		}}
	}

	if rc.Spec.Isolated {
		sts.Spec.Template.Spec.Affinity = &corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
					TopologyKey: "kubernetes.io/hostname",
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: label,
					},
				}},
			},
		}
	}

	if rc.Spec.MasterSchedule {
		sts.Spec.Template.Spec.Tolerations = []corev1.Toleration{{
			Effect: corev1.TaintEffectNoSchedule,
			Key:    "node-role.kubernetes.io/master",
		}}
	}

	return sts
}
