package tencentcloud

import (
	"testing"

	"github.com/hashicorp/terraform/helper/resource"
)

func TestAccTencentCloudDataSourceNetworkInterface_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: TestAccTencentCloudDataSourceNetworkInterfaceConfig_basic,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTencentCloudDataSourceID("data.tencentcloud_network_interface.foo"),
				),
			},
		},
	})
}

const TestAccTencentCloudDataSourceNetworkInterfaceConfig_basic = `
data "tencentcloud_network_interface" "foo" {
}
`
