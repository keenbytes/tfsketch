module "ref-1-to-external-mod2" {
  source = "external-tf-module-2"
  version = "0.1.0"
}

module "ref-2-to-external-mod2" {
  source = "external-tf-module-2"
  version = "0.1.0"
}

resource "resource_type_different" "resource_name_ing2_1" {
  name = "name-ing1-2"
  key1 = "value1_2"
  key2 = "value2_2"
}

module "ref-1-to-external-mod-modules-4" {
  source = "external-tf-module-4"
  version = "0.1.0"
}
