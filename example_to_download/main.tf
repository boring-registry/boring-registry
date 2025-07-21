terraform {
  required_version = ">= 1.0"
}

module "example_module" {
  source = "terraform-registry.stage.dp.confluent.io/boring_registry_test/example/aws"
  version = "2.0.0"
}

output "module_outputs" {
  description = "Outputs from the example module"
  value = module.example_module
}
