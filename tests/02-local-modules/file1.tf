resource "type" "type-name-11" {
  name = "name-11"
}

resource "type" "type-name-12" {
  for_each = var.value1
  name = "name-12"
}

module "sub1-1" {
  source = "./sub1"
  suffix = "11"
}

module "sub1-2" {
  for_each = var.value2

  source = "./sub1"
  suffix = "12"
}

module "sub4" {
  source = "./sub4"
  suffix = "11"
}
