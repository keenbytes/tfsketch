variable "suffix" {
  type = string
}

module "sub4sub1" {
  source = "./sub4sub1"
  suffix = var.suffix
}
