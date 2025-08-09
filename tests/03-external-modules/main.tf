resource "type" "root-1-1" {
  name = "name-root-1-1"
}

module "mod1-1" {
  source = "external-module-1"
  version = "0.0.1"
}

module "mod2-1" {
  source = "external-module-2"
  version = "0.0.1"
}

module "mod2sub1-1" {
  source = "external-module-2//modules/sub1"
  version = "0.0.1"
}
