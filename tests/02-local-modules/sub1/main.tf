variable "suffix" {
  type = string
}

resource "type" "sub1" {
  name = "name-sub1-${var.suffix}"
}
