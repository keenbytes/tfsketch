module "ing2-mod2-1" {
  source = "external-tf-module-2"
  version = "0.1.0"
}

module "ing2-mod2-2" {
  source = "external-tf-module-2"
  version = "0.1.0"
}

resource "resource_type_different" "resource_name_ing2_1" {
  name = "name-ing1-2"
  key1 = "value1_2"
  key2 = "value2_2"
}

module "ing2-mod4" {
  source = "external-tf-module-4"
  version = "0.1.0"
}
