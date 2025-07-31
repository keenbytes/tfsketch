resource "resource_type" "acc2_1" {
  name = "name-acc2-1"
}

resource "another_resource" "acc2" {
  key1 = "value1"
}

resource "resource_type" "acc2_2" {
  name = "name-acc2-2"
}

module "mod1-1" {
  source = "./modules/module-1"
}

module "mod1-2" {
  source = "./modules/module-1"
}
