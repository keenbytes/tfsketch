resource "type" "extmod-1-1" {
  name = "name-extmod-1-1"
}

resource "type" "extmod-1-2" {
  name = "name-extmod-1-2"
  for_each = var.value6
}

resource "nevermind" "nevermind-1" {
  name = "name-nevermind-1"
}
