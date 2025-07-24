resource "resource_type" "ing1_1" {
  name = "name-ing1-1"
  key1 = "value1"
  key2 = "value2"
}

resource "another_resource" "resource_name_ing1" {
  key1 = "value1"
}

resource "resource_type" "ing1_2" {
  name = "name-ing1-2"
  key1 = "value1_2"
  key2 = "value2_2"
}
