package tencentcloud

import (
	"fmt"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/zqfan/tencentcloud-sdk-go/common"
	vpc "github.com/zqfan/tencentcloud-sdk-go/services/vpc/unversioned"
)

func dataSourceTencentCloudNetworkInterface() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceTencentCloudNetworkInterfaceRead,

		Schema: map[string]*schema.Schema{
			"id": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"vpc_id": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"name": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"instance_id": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"ips": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"primary": {
							Type:     schema.TypeBool,
							Computed: true,
						},
						"private_ip": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"public_ip": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"eip_id": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
		},
	}
}

func dataSourceTencentCloudNetworkInterfaceRead(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*TencentCloudClient).vpcConn

	req := vpc.NewDescribeNetworkInterfacesRequest()
	if v, ok := d.GetOk("id"); ok {
		req.NetworkInterfaceId = common.StringPtr(v.(string))
	}
	if v, ok := d.GetOk("vpc_id"); ok {
		req.VpcId = common.StringPtr(v.(string))
	}
	if v, ok := d.GetOk("name"); ok {
		req.EniName = common.StringPtr(v.(string))
	}
	if v, ok := d.GetOk("instance_id"); ok {
		req.InstanceId = common.StringPtr(v.(string))
	}

	resp, err := conn.DescribeNetworkInterfaces(req)
	if err != nil {
		return err
	}
	if _, ok := err.(*common.APIError); ok {
		return fmt.Errorf("DescribeNetworkInterfaces error: %v", err)
	}
	if *resp.Data.TotalNum == 0 {
		return fmt.Errorf("No matching network interface found.")
	}

	eni := resp.Data.Data[0]
	d.SetId(*eni.NetworkInterfaceId)
	d.Set("name", *eni.EniName)
	d.Set("vpc_id", *eni.VpcId)
	d.Set("instance_id", *eni.InstanceSet.InstanceId)
	ips := []map[string]interface{}{}
	for _, privateIp := range eni.PrivateIpAddressesSet {
		ip := map[string]interface{}{
			"primary":    *privateIp.Primary,
			"private_ip": *privateIp.PrivateIpAddress,
			"public_ip":  *privateIp.WanIp,
			"eip_id":     *privateIp.EipId,
		}
		ips = append(ips, ip)
	}
	d.Set("ips", ips)
	return nil
}
