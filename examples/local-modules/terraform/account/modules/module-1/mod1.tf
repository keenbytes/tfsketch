locals {
  some_variable = "local_value"
  another_local2 = "name-mod1-1"
}

resource "resource_type" "resource_name_mod1_1" {
  name = local.another_local2
  key1 = "value1"
  key2 = "value2"
}

resource "resource_type" "resource_name_mod1_2" {
  name = "${local.some_variable}-name-mod1-2"
  key1 = "value1_2"
  key2 = "value2_2"
}

