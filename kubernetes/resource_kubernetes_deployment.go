package kubernetes

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pkgApi "k8s.io/apimachinery/pkg/types"
	kubernetes "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

func resourceKubernetesDeployment() *schema.Resource {
	return &schema.Resource{
		Create: resourceKubernetesDeploymentCreate,
		Read:   resourceKubernetesDeploymentRead,
		Exists: resourceKubernetesDeploymentExists,
		Update: resourceKubernetesDeploymentUpdate,
		Delete: resourceKubernetesDeploymentDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		SchemaVersion: 2,
		MigrateState:  resourceKubernetesDeploymentStateUpgrader,

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(10 * time.Minute),
			Update: schema.DefaultTimeout(10 * time.Minute),
			Delete: schema.DefaultTimeout(10 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"metadata": namespacedMetadataSchema("deployment", true),
			"name": {
				Type:     schema.TypeString,
				Optional: true,
				Removed:  "To better match the Kubernetes API, the name attribute should be configured under the metadata block. Please update your Terraform configuration.",
			},
			"spec": {
				Type:        schema.TypeList,
				Description: "Spec defines the specification of the desired behavior of the deployment. More info: http://releases.k8s.io/HEAD/docs/devel/api-conventions.md#spec-and-status",
				Required:    true,
				MaxItems:    1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"min_ready_seconds": {
							Type:        schema.TypeInt,
							Description: "Minimum number of seconds for which a newly created pod should be ready without any of its container crashing, for it to be considered available. Defaults to 0 (pod will be considered available as soon as it is ready)",
							Optional:    true,
							Default:     0,
						},
						"paused": {
							Type:        schema.TypeBool,
							Description: "Indicates that the deployment is paused.",
							Optional:    true,
							Default:     false,
						},
						"progress_deadline_seconds": {
							Type:        schema.TypeInt,
							Description: "The maximum time in seconds for a deployment to make progress before it is considered to be failed. The deployment controller will continue to process failed deployments and a condition with a ProgressDeadlineExceeded reason will be surfaced in the deployment status. Note that progress will not be estimated during the time a deployment is paused. Defaults to 600s.",
							Optional:    true,
							Default:     600,
						},
						"replicas": {
							Type:        schema.TypeInt,
							Description: "The number of desired replicas. Defaults to 1. More info: http://kubernetes.io/docs/user-guide/replication-controller#what-is-a-replication-controller",
							Optional:    true,
							Default:     1,
						},
						"revision_history_limit": {
							Type:        schema.TypeInt,
							Description: "The number of old ReplicaSets to retain to allow rollback. Defaults to 10.",
							Optional:    true,
							Default:     10,
						},
						"selector": {
							Type:        schema.TypeMap,
							Description: "A label query over pods that should match the Replicas count. If Selector is empty, it is defaulted to the labels present on the Pod template. Label keys and values that must match in order to be controlled by this deployment, if empty defaulted to labels on Pod template. More info: http://kubernetes.io/docs/user-guide/labels#label-selectors",
							Optional:    true,
							Computed:    true,
						},
						"strategy": {
							Type:        schema.TypeList,
							Optional:    true,
							Computed:    true,
							Description: "Update strategy. One of RollingUpdate, Destroy. Defaults to RollingDate",
							MaxItems:    1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"type": {
										Type:        schema.TypeString,
										Optional:    true,
										Computed:    true,
										Description: "Update strategy",
									},
									"rolling_update": {
										Type:        schema.TypeList,
										Description: "rolling update",
										Optional:    true,
										Computed:    true,
										MaxItems:    1,
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"max_surge": {
													Type:        schema.TypeString,
													Description: "max surge",
													Optional:    true,
													Default:     1,
												},
												"max_unavailable": {
													Type:        schema.TypeString,
													Description: "max unavailable",
													Optional:    true,
													Default:     1,
												},
											},
										},
									},
								},
							},
						},
						"template": {
							Type:        schema.TypeList,
							Description: "Template describes the pods that will be created.",
							Required:    true,
							MaxItems:    1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"metadata": metadataSchema("deploymentSpec", true),
									"spec": &schema.Schema{
										Type:        schema.TypeList,
										Description: "Template describes the pods that will be created.",
										Required:    true,
										MaxItems:    1,
										Elem: &schema.Resource{
											Schema: podSpecFields(true),
										},
									},
									"active_deadline_seconds":          relocatedAttribute("active_deadline_seconds"),
									"container":                        relocatedAttribute("container"),
									"dns_policy":                       relocatedAttribute("dns_policy"),
									"host_ipc":                         relocatedAttribute("host_ipc"),
									"host_network":                     relocatedAttribute("host_network"),
									"host_pid":                         relocatedAttribute("host_pid"),
									"hostname":                         relocatedAttribute("hostname"),
									"init_container":                   relocatedAttribute("init_container"),
									"node_name":                        relocatedAttribute("node_name"),
									"node_selector":                    relocatedAttribute("node_selector"),
									"restart_policy":                   relocatedAttribute("restart_policy"),
									"security_context":                 relocatedAttribute("security_context"),
									"service_account_name":             relocatedAttribute("service_account_name"),
									"automount_service_account_token":  relocatedAttribute("automount_service_account_token"),
									"subdomain":                        relocatedAttribute("subdomain"),
									"termination_grace_period_seconds": relocatedAttribute("termination_grace_period_seconds"),
									"volume": relocatedAttribute("volume"),
								},
							},
						},
					},
				},
			},
		},
	}
}

func relocatedAttribute(name string) *schema.Schema {
	s := &schema.Schema{
		Type:     schema.TypeString,
		Optional: true,
		Removed:  fmt.Sprintf("%s has been relocated to [resource]/spec/template/spec/%s. Please update your Terraform config.", name, name),
	}
	return s
}

func resourceKubernetesDeploymentCreate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*kubernetes.Clientset)

	metadata := expandMetadata(d.Get("metadata").([]interface{}))
	spec, err := expandDeploymentSpec(d.Get("spec").([]interface{}))
	if err != nil {
		return err
	}
	if metadata.Namespace == "" {
		metadata.Namespace = "default"
	}

	deployment := v1beta1.Deployment{
		ObjectMeta: metadata,
		Spec:       spec,
	}

	log.Printf("[INFO] Creating new deployment: %#v", deployment)
	out, err := conn.ExtensionsV1beta1().Deployments(metadata.Namespace).Create(&deployment)
	if err != nil {
		return fmt.Errorf("Failed to create deployment: %s", err)
	}

	d.SetId(buildId(out.ObjectMeta))

	log.Printf("[DEBUG] Waiting for deployment %s to schedule %d replicas",
		d.Id(), *out.Spec.Replicas)
	// 10 mins should be sufficient for scheduling ~10k replicas
	err = resource.Retry(d.Timeout(schema.TimeoutCreate),
		waitForDeploymentReplicasFunc(conn, out.GetNamespace(), out.GetName()))
	if err != nil {
		return err
	}
	// We could wait for all pods to actually reach Ready state
	// but that means checking each pod status separately (which can be expensive at scale)
	// as there's no aggregate data available from the API

	log.Printf("[INFO] Submitted new deployment: %#v", out)

	return resourceKubernetesDeploymentRead(d, meta)
}

func resourceKubernetesDeploymentRead(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*kubernetes.Clientset)

	namespace, name, err := idParts(d.Id())
	log.Printf("[INFO] Reading deployment %s", name)
	deployment, err := conn.ExtensionsV1beta1().Deployments(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		log.Printf("[DEBUG] Received error: %#v", err)
		return err
	}
	log.Printf("[INFO] Received deployment: %#v", deployment)

	deployment.ObjectMeta.Labels = reconcileTopLevelLabels(
		deployment.ObjectMeta.Labels,
		expandMetadata(d.Get("metadata").([]interface{})),
		expandMetadata(d.Get("spec.0.template.0.metadata").([]interface{})),
	)
	err = d.Set("metadata", flattenMetadata(deployment.ObjectMeta, d))
	if err != nil {
		return err
	}

	spec, err := flattenDeploymentSpec(deployment.Spec, d)
	if err != nil {
		return err
	}

	err = d.Set("spec", spec)
	if err != nil {
		return err
	}

	return nil
}

func resourceKubernetesDeploymentUpdate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*kubernetes.Clientset)

	namespace, name, err := idParts(d.Id())

	ops := patchMetadata("metadata.0.", "/metadata/", d)

	if d.HasChange("spec") {
		spec, err := expandDeploymentSpec(d.Get("spec").([]interface{}))
		if err != nil {
			return err
		}

		ops = append(ops, &ReplaceOperation{
			Path:  "/spec",
			Value: spec,
		})
	}
	data, err := ops.MarshalJSON()
	if err != nil {
		return fmt.Errorf("Failed to marshal update operations: %s", err)
	}
	log.Printf("[INFO] Updating deployment %q: %v", name, string(data))
	out, err := conn.ExtensionsV1beta1().Deployments(namespace).Patch(name, pkgApi.JSONPatchType, data)
	if err != nil {
		return fmt.Errorf("Failed to update deployment: %s", err)
	}
	log.Printf("[INFO] Submitted updated deployment: %#v", out)

	err = resource.Retry(d.Timeout(schema.TimeoutUpdate),
		waitForDeploymentReplicasFunc(conn, namespace, name))
	if err != nil {
		return err
	}

	return resourceKubernetesDeploymentRead(d, meta)
}

func resourceKubernetesDeploymentDelete(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*kubernetes.Clientset)

	namespace, name, err := idParts(d.Id())
	log.Printf("[INFO] Deleting deployment: %#v", name)

	// Drain all replicas before deleting
	var ops PatchOperations
	ops = append(ops, &ReplaceOperation{
		Path:  "/spec/replicas",
		Value: 0,
	})
	data, err := ops.MarshalJSON()
	if err != nil {
		return err
	}
	_, err = conn.ExtensionsV1beta1().Deployments(namespace).Patch(name, pkgApi.JSONPatchType, data)
	if err != nil {
		return err
	}

	// Wait until all replicas are gone
	err = resource.Retry(d.Timeout(schema.TimeoutDelete),
		waitForDeploymentReplicasFunc(conn, namespace, name))
	if err != nil {
		return err
	}

	policy := metav1.DeletePropagationForeground
	err = conn.ExtensionsV1beta1().Deployments(namespace).Delete(name, &metav1.DeleteOptions{
		PropagationPolicy: &policy,
	})
	if err != nil {
		return err
	}

	log.Printf("[INFO] Deployment %s deleted", name)

	d.SetId("")
	return nil
}

func resourceKubernetesDeploymentExists(d *schema.ResourceData, meta interface{}) (bool, error) {
	conn := meta.(*kubernetes.Clientset)

	namespace, name, err := idParts(d.Id())
	log.Printf("[INFO] Checking deployment %s", name)
	_, err = conn.ExtensionsV1beta1().Deployments(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		if statusErr, ok := err.(*errors.StatusError); ok && statusErr.ErrStatus.Code == 404 {
			return false, nil
		}
		log.Printf("[DEBUG] Received error: %#v", err)
	}
	return true, err
}

func waitForDeploymentReplicasFunc(conn *kubernetes.Clientset, ns, name string) resource.RetryFunc {
	return func() *resource.RetryError {
		deployment, err := conn.ExtensionsV1beta1().Deployments(ns).Get(name, metav1.GetOptions{})
		if err != nil {
			return resource.NonRetryableError(err)
		}

		desiredReplicas := *deployment.Spec.Replicas
		log.Printf("[DEBUG] Current number of labelled replicas of %q: %d (of %d)\n",
			deployment.GetName(), deployment.Status.Replicas, desiredReplicas)

		if deployment.Status.Replicas == desiredReplicas {
			return nil
		}

		return resource.RetryableError(fmt.Errorf("Waiting for %d replicas of %q to be scheduled (%d)",
			desiredReplicas, deployment.GetName(), deployment.Status.Replicas))
	}
}

func resourceKubernetesDeploymentStateUpgrader(
	v int, is *terraform.InstanceState, meta interface{}) (*terraform.InstanceState, error) {
	if is.Empty() {
		log.Println("[DEBUG] Empty InstanceState; nothing to migrate.")
		return is, nil
	}

	var err error

	switch v {
	case 0:
		log.Println("[INFO] Found Kubernetes Deployment State v0; migrating to v1")
		is, err = migrateStateV0toV1(is)
	case 1:
		log.Println("[INFO] Found Kubernetes Deployment State v1; migrating to v2")
		is, err = migrateStateV1toV2(is)

	default:
		return is, fmt.Errorf("Unexpected schema version: %d", v)
	}

	return is, err
}

// This deployment resource originally had the podSpec directly below spec.template level
// This migration moves the state to spec.template.spec match the Kubernetes documented structure
func migrateStateV0toV1(is *terraform.InstanceState) (*terraform.InstanceState, error) {
	log.Printf("[DEBUG] Attributes before migration: %#v", is.Attributes)

	newTemplate := make(map[string]string)

	for k, v := range is.Attributes {
		log.Println("[DEBUG] - checking attribute for state upgrade: ", k, v)
		if strings.HasPrefix(k, "name") {
			// don't clobber an existing metadata.0.name value
			if _, ok := is.Attributes["metadata.0.name"]; ok {
				continue
			}

			newK := "metadata.0.name"

			newTemplate[newK] = v
			log.Printf("[DEBUG] moved attribute %s -> %s ", k, newK)
			delete(is.Attributes, k)

		} else if !strings.HasPrefix(k, "spec.0.template") {
			continue

		} else if strings.HasPrefix(k, "spec.0.template.0.spec") || strings.HasPrefix(k, "spec.0.template.0.metadata") {
			continue

		} else {
			newK := strings.Replace(k, "spec.0.template.0", "spec.0.template.0.spec.0", 1)

			newTemplate[newK] = v
			log.Printf("[DEBUG] moved attribute %s -> %s ", k, newK)
			delete(is.Attributes, k)
		}
	}

	for k, v := range newTemplate {
		is.Attributes[k] = v
	}

	log.Printf("[DEBUG] Attributes after migration: %#v", is.Attributes)
	return is, nil
}

// Add schema fields: paused, progress_deadline_seconds
func migrateStateV1toV2(is *terraform.InstanceState) (*terraform.InstanceState, error) {
	log.Printf("[DEBUG] Attributes before migration: %#v", is.Attributes)

	is.Attributes["spec.0.paused"] = "false"
	is.Attributes["spec.0.progress_deadline_seconds"] = "600"

	log.Printf("[DEBUG] Attributes after migration: %#v", is.Attributes)
	return is, nil
}
