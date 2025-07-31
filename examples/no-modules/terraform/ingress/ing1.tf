resource "resource_type" "ing1_1" {
  name = "name-ing1-1"
  key1 = "value1"
  key2 = "value2"
}

resource "resource_type" "ing1_2" {
  name = "name-ing1-2"
  key1 = "value1_2"
  key2 = "value2_2"
}

resource "non_important" "nevermind" {
  key1 = "value1"
}
