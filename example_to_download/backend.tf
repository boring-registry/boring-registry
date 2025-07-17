terraform {
  backend "s3" {
    bucket = "private-terraform-registry-755363985185-us-west-2"
    key    = "terraform/example-test-public-url/terraform.tfstate"
    region = "us-west-2"
  }
} 
