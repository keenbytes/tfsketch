resource "resource_type" "acc1_1" {
  name = local.name_acc1_1
}

resource "resource_type" "acc1_2" {
  name = "${local.some_variable}-name-acc1-2"
}

resource "non_important_resource" "nevermind" {
  key1 = "value1"
}

module "accsubdir_1" {
  source = "./accsubdir"
}

module "accsubdir_2" {
  source = "./accsubdir/"
}
