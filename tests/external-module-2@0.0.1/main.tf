resource "type" "extmod-2-1" {
  name = "name-extmod-2-1"
}

resource "type" "extmod-2-2" {
  name = "name-extmod-2-2"
}

module "subm2" {
  source = "./subm2"
  version = "0.0.1"
  for_each = var.value7
}
