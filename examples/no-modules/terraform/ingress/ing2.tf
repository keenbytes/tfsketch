module "ing2-mod2-1" {
  source = "external-tf-module-2"
  version = "0.1.0"
}

module "ing2-mod2-2" {
  source = "external-tf-module-2"
  version = "0.1.0"
}

module "ing2-mod4" {
  source = "external-tf-module-4"
  version = "0.1.0"
}
