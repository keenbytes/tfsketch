variable "suffix" {
  type = string
}

resource "type" "sub4sub1sub1" {
  name = "name-sub4sub1sub1-${var.suffix}"
}
