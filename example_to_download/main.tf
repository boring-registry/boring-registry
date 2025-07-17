terraform {
  required_version = ">= 1.0"
}

# Call the example module from terraform-registry
module "example_module" {
  source = "terraform-registry.stage.dp.confluent.io/boring_registry_test/example/aws"
  version = "3.0.1"
}

# Output any values from the module
output "module_outputs" {
  description = "Outputs from the example module"
  value = module.example_module
}
