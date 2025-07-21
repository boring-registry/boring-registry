provider "aws" {
  region = "us-west-2"
  
  default_tags {
    tags = {
      Environment = "nonprod"
      Project     = "boring-registry-validation-semaphore"
      ManagedBy   = "terraform"
    }
  }
} 
