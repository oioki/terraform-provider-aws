package aws

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/wafregional"
	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/helper/schema"
)

func resourceAwsWafRegionalWebAclAssociation() *schema.Resource {
	return &schema.Resource{
		Create: resourceAwsWafRegionalWebAclAssociationCreate,
		Read:   resourceAwsWafRegionalWebAclAssociationRead,
		Delete: resourceAwsWafRegionalWebAclAssociationDelete,

		Schema: map[string]*schema.Schema{
			"web_acl_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"resource_arn": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
		},
	}
}

func resourceAwsWafRegionalWebAclAssociationCreate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).wafregionalconn

	log.Printf(
		"[INFO] Creating WAF Regional Web ACL association: %s => %s",
		d.Get("web_acl_id").(string),
		d.Get("resource_arn").(string))

	params := &wafregional.AssociateWebACLInput{
		WebACLId:    aws.String(d.Get("web_acl_id").(string)),
		ResourceArn: aws.String(d.Get("resource_arn").(string)),
	}

	// create association and wait on retryable error
	// no response body
	var err error
	err = resource.Retry(2*time.Minute, func() *resource.RetryError {
		_, err = conn.AssociateWebACL(params)
		if err != nil {
			if isAWSErr(err, wafregional.ErrCodeWAFUnavailableEntityException, "") {
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Store association id
	d.SetId(fmt.Sprintf("%s:%s", *params.WebACLId, *params.ResourceArn))

	return nil
}

func resourceAwsWafRegionalWebAclAssociationRead(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).wafregionalconn

	webAclId, resourceArn := resourceAwsWafRegionalWebAclAssociationParseId(d.Id())

	/*
		AWS does not return all resource types on ListResourcesForWebACL, default
		behavior is to only return ALB resources. Make call for each possible
		resource type
	*/
	found := false
	RESOURCE_TYPES := []string{"APPLICATION_LOAD_BALANCER", "API_GATEWAY"}
	for _, resource := range RESOURCE_TYPES {
		// List all resources for Web ACL of resource type to see if theres a match
		params := &wafregional.ListResourcesForWebACLInput{
			WebACLId:     aws.String(webAclId),
			ResourceType: aws.String(resource),
		}
		resp, err := conn.ListResourcesForWebACL(params)
		if err != nil {
			return err
		}

		for _, listResourceArn := range resp.ResourceArns {
			if resourceArn == *listResourceArn {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		log.Printf("[WARN] WAF Regional Web ACL association (%s) not found, removing from state", d.Id())
		d.SetId("")
	}

	return nil
}

func resourceAwsWafRegionalWebAclAssociationDelete(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).wafregionalconn

	_, resourceArn := resourceAwsWafRegionalWebAclAssociationParseId(d.Id())

	log.Printf("[INFO] Deleting WAF Regional Web ACL association: %s", resourceArn)

	params := &wafregional.DisassociateWebACLInput{
		ResourceArn: aws.String(resourceArn),
	}

	// If action successful HTTP 200 response with an empty body
	_, err := conn.DisassociateWebACL(params)
	return err
}

func resourceAwsWafRegionalWebAclAssociationParseId(id string) (webAclId, resourceArn string) {
	parts := strings.SplitN(id, ":", 2)
	webAclId = parts[0]
	resourceArn = parts[1]
	return
}
