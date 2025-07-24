locals {
  some_variable = "local_value"
  another_local = "name-acc1-1"
}

resource "resource_type" "acc1_1" {
  name = local.another_local
  key1 = "value1"
  key2 = "value2"
}

resource "another_resource" "resource_name_acc1" {
  key1 = "value1"
}

resource "resource_type" "acc1_2" {
  name = "${local.some_variable}-name-acc1-2"
  key1 = "value1_2"
  key2 = "value2_2"
}

