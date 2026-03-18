env "local" {
  src = "file://schema.sql"
  url = "postgres://croncontrol:croncontrol@localhost:5435/croncontrol?sslmode=disable"
  dev = "docker://postgres/16/dev"

  migration {
    dir = "file://migrations"
  }
}

env "ci" {
  src = "file://schema.sql"
  dev = "docker://postgres/16/dev"

  migration {
    dir = "file://migrations"
  }
}
