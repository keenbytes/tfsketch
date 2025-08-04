variable "suffix" {
  type = string
}

resource "type" "sub3" {
  name = "name-sub3-${var.suffix}"
}
