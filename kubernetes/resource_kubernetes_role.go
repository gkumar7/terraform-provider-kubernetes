package kubernetes

import (
	"github.com/hashicorp/terraform/helper/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"log"
)

func resourceKubernetesRole() *schema.Resource {
	return &schema.Resource{
		Create: resourceKubernetesRoleCreate,
		Read:   resourceKubernetesRoleRead,
		Update: resourceKubernetesRoleUpdate,
		Delete: resourceKubernetesRoleDelete,
		Exists: resourceKubernetesRoleExists,

		Schema: map[string]*schema.Schema{
			"metadata": namespacedMetadataSchema("role", true),
			"policy_rule": {
				Type:        schema.TypeList,
				Description: "list of policy rules",
				Optional:    true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"api_groups": {
							Type:        schema.TypeList,
							Description: "",
							Optional:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
						},
						"non_resource_urls": {
							Type:        schema.TypeList,
							Description: "",
							Optional:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
						},
						"resource_names": {
							Type:        schema.TypeList,
							Description: "",
							Optional:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
						},
						"resources": {
							Type:        schema.TypeList,
							Description: "",
							Optional:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
						},
						"verbs": {
							Type:        schema.TypeList,
							Description: "",
							Optional:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
						},
					},
				},
			},
		},
	}
}

func resourceKubernetesRoleCreate(d *schema.ResourceData, meta interface{}) error {
	return nil
}

func resourceKubernetesRoleRead(d *schema.ResourceData, meta interface{}) error {
	return nil
}

func resourceKubernetesRoleUpdate(d *schema.ResourceData, meta interface{}) error {
	return nil
}

func resourceKubernetesRoleDelete(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*kubernetes.Clientset)

	namespace, name, err := idParts(d.Id())
	if err != nil {
		return err
	}

	log.Printf("[INFO] Deleting role: %#v", name)
	err = conn.RbacV1beta1().Roles(namespace).Delete(name, &metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	log.Printf("[INFO] Role %s deleted", name)

	d.SetId("")
	return nil
}

func resourceKubernetesRoleExists(d *schema.ResourceData, meta interface{}) (bool, error) {
	conn := meta.(*kubernetes.Clientset)

	namespace, name, err := idParts(d.Id())
	if err != nil {
		return false, err
	}

	log.Printf("[INFO] Checking role %s", name)
	_, err = conn.RbacV1beta1().Roles(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		if statusErr, ok := err.(*errors.StatusError); ok && statusErr.ErrStatus.Code == 404 {
			return false, nil
		}
		log.Printf("[DEBUG] Received error: %#v", err)
	}
	return true, err
}
