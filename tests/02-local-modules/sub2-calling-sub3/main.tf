variable "suffix" {
  type = string
}

resource "type" "sub2" {
  name = "name-sub2-${var.suffix}"
}

module "sub3" {
  source = "../sub3"
}
