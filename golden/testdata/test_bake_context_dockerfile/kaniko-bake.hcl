target "app" {
  context     = "app"
  dockerfile  = "Dockerfile"
  destination = ["registry.example.com/app:latest"]
}

target "tools" {
  context     = "tools"
  dockerfile  = "Dockerfile.tools"
  destination = ["registry.example.com/tools:latest"]
}
