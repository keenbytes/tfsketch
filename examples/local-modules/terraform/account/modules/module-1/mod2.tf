resource "resource_type" "resource_name_mod2_1" {
  name = "name-mod2-1"
  key1 = "value1"
  key2 = "value2"
}

resource "another_resource" "resource_name_mod2" {
  key1 = "value1"
}

resource "resource_type" "resource_name_mod2_2" {
  for_each = local.something

  name = "name-mod2-2"
  key1 = "value1_2"
  key2 = "value2_2"
}

