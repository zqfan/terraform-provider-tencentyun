---
layout: "tencentcloud"
page_title: "TencentCloud: tencentcloud_network_interface"
sidebar_current: "docs-tencentcloud-datasource-network-interface"
description: |-
  Provides details about a specific network interface.
---

# tencentcloud_network_interface

`tencentcloud_network_interface` provides details about a specific network interface.

## Example Usage


```hcl

data "tencentcloud_network_interface" "foo" {
  vpc_id = "vpc-kg60ct5z"
}

resource "tencentcloud_eip" "foo" {
  name = "terraform-eip-test"
}

resource "tencentcloud_eip_association" "foo" {
  eip_id               = "${tencentcloud_eip.foo.id}"
  network_interface_id = "${data.tencentcloud_network_interface.foo.id}"
  private_ip           = "${data.tencentcloud_network_interface.foo.ips.0.private_ip}"
}
```

## Argument Reference

The following arguments are supported:

* `id` - (Optional) The ID of the specific network interface to retrieve.
* `name` - (Optional) Network interface name. Fuzzy search is supported.
* `vpc_id` - (Optional) The VPC ID which the target network interface belongs to.
* `instance_id` - (Optional) The instance ID which the target network interface is binded with.

## Attributes Reference

The following attribute is additionally exported:

* `id` - Network interface's ID.
* `name` - Network interface's name.
* `vpc_id` - The VPC ID this network interface belongs to.
* `instance_id` - The instance ID this network interface is binded with.
* `ips` - A list of IPs this network interface has, including the following attributes:
	* `primary` - (Bool) Whether this IP is primary.
	* `private_ip` - The private IP of this network interface.
	* `public_ip` - The corresponding public IP to the private IP.
	* `eip_id` - The associated Elastic IP ID.
