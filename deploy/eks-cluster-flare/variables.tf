variable "cluster_name" {
  default = "macos-cluster"
  description = "Name of EKS cluster where you want to add nodes"
}

variable "iam_instance_profile" {
  default = "MacMetal1EKSWorkerNodeRole"
}

variable "region" {
  default = "us-east-1"
}

variable "excluded_azs" {
  type = list(string)
  default = [
    "use1-az3"]
}

variable "node_ami" {
  // Flare base AMI with xcode 12.3 installed
  default = "ami-026383ea21564b248"
  validation {
    condition = length(var.node_ami) > 4 && substr(var.node_ami, 0, 4) == "ami-"
    error_message = "The node_ami value must be a valid AMI id, starting with \"ami-\"."
  }
}

variable "kubelet_url" {
  default = ""
}

variable "kube_proxy_url" {
  default = ""
}

variable "procri_url" {
  default = ""
}

variable "nodes_number" {
  type = number
  default = 1
}

variable "map_accounts" {
  description = "Additional AWS account numbers to add to the aws-auth configmap."
  type = list(string)

  default = [
  ]
}

variable "map_roles" {
  description = "Additional IAM roles to add to the aws-auth configmap."
  type = list(object({
    rolearn = string
    username = string
    groups = list(string)
  }))

  default = [
    {
      rolearn = "arn:aws:iam::387435531342:role/MacMetal1EKSWorkerNodeRole"
      username = "system:node:{{EC2PrivateDNSName}}"
      groups = [
        "system:masters",
        "system:nodes",
        "system:bootstrappers"]
    }
  ]
}

variable "map_users" {
  description = "Additional IAM users to add to the aws-auth configmap."
  type = list(object({
    userarn = string
    username = string
    groups = list(string)
  }))

  default = [
    {
      userarn = "arn:aws:iam::387435531342:user/pawel@elotl.co"
      username = "pawel@elotl.co"
      groups = [
        "system:masters"]
    },
    {
      userarn = "arn:aws:iam::387435531342:user/henry@elotl.co"
      username = "henry@elotl.co"
      groups = [
        "system:masters"]
    },
    {
      userarn = "arn:aws:iam::387435531342:user/madhuri@elotl.co"
      username = "madhuri@elotl.co"
      groups = [
        "system:masters"]
    },
    {
      userarn = "arn:aws:iam::387435531342:user/andrey"
      username = "andrey"
      groups = [
        "system:masters"]
    },
  ]
}
