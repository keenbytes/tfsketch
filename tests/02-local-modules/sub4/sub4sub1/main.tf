variable "suffix" {
  type = string
}

module "sub4sub1sub1" {
  source = "./sub4sub1sub1"
  suffix = var.suffix
}

resource "type" "sub4sub1-1" {
  name = "name-sub4sub1-1-${var.suffix}"
}

resource "type" "sub4sub1-2" {
  name = "name-sub4sub1-2-${var.suffix}"
}
