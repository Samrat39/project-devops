variable "cluster_name" {
  default = "k8s_cluster"
  type = string
}

variable "cluster_version" {
  type = string
}

variable "vpc_id" {
  type = string
}

variable "subnet_ids" {
  type = list(string)
}

variable "node_groups" {
  type = map(object({
    instance_type = list(string)
    capacity_type = string
    scaling_config = object({
      desired_size = number
    max_size = number
    min_size = number
    })
  }))
}

