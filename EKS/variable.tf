variable "vpc_cidr" {
  type    = string
  default = "10.0.0.0/16"
}

variable "availability_zones" {
  type = list(string)
  default = [ "us-east-1a" , "us-east-1b" ]
}

variable "private_subnet_cidrs" {
  type    = list(string)
  default = ["10.0.1.0/24" , "10.0.2.0/24"]
}


variable "public_subnet_cidrs" {
  type    = list(string)
  default = ["10.0.3.0/24" ,  "10.0.4.0/24"]
}

variable "cluster_name" {
  type    = string
  default = "abc"
}

variable "cluster_version" {
  type    = string
  default = "1.30"
}

variable "node_groups" {
  type = map(object({
    instance_type  = list(string)
    capacity_type  = string
    scaling_config = object({
      desired_size = number
      max_size     = number
      min_size     = number
    })
  }))

  default = {
    ng1 = {
      instance_type  = ["t2.micro"]
      capacity_type  = "ON_DEMAND"
      scaling_config = {
        desired_size = 2
        max_size     = 2
        min_size     = 1
      }
    }
  }
}
