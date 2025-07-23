resource "resource_type" "resource_name_acc2_1" {
  name = "name-acc2-1"
  key1 = "value1"
  key2 = "value2"
}

resource "another_resource" "resource_name_acc2" {
  key1 = "value1"
}

resource "resource_type" "resource_name_acc2_2" {
  name = "name-acc2-2"
  key1 = "value1_2"
  key2 = "value2_2"
}

module "mod1-1" {
  source = "./modules/module-1"
}

module "mod1-2" {
  source = "./modules/module-1"
}
