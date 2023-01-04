terraform {
  backend "s3" {
    bucket = "tf-state-macos"
    key = "macos_cluster.tfstate"
    region = "us-east-1"
    profile = "flare"
  }
  required_providers {
    external = {
      version = "~> 1.2"
    }
    random = {
      version = "~> 2.3"
    }
    template = {
      version = "~> 2.1"
    }
    tls = {
      version = "~> 2.2"
    }
    aws = {
      version = "~> 3.0"
    }
    null = {
      version = "~> 2.1"
    }
    kubernetes = {
      version = "~> 1.11"
    }
  }
}

provider "aws" {
  region = var.region
  profile = "flare"
}

data "aws_eks_cluster" "cluster" {
  name = module.eks.cluster_id
}

data "aws_eks_cluster_auth" "cluster" {
  name = module.eks.cluster_id
}

provider "kubernetes" {
  host = data.aws_eks_cluster.cluster.endpoint
  cluster_ca_certificate = base64decode(data.aws_eks_cluster.cluster.certificate_authority.0.data)
  token = data.aws_eks_cluster_auth.cluster.token
  load_config_file = false
}

data "aws_availability_zones" "available" {
  state = "available"
  exclude_zone_ids = var.excluded_azs

  filter {
    name = "opt-in-status"
    values = [
      "opt-in-not-required"]
  }
}

locals {
  k8s_cluster_tags = {
    "Name" = var.cluster_name
    "kubernetes.io/cluster/${var.cluster_name}" = "owned"
  }
}

module "vpc" {
  source = "terraform-aws-modules/vpc/aws"
  version = "2.77.0"

  name = "${local.cluster_name}-vpc"
  cidr = "172.16.0.0/16"
  azs = data.aws_availability_zones.available.names
  private_subnets = [
    "172.16.1.0/24",
    "172.16.2.0/24",
    "172.16.3.0/24"]
  public_subnets = [
    "172.16.4.0/24",
    "172.16.5.0/24",
    "172.16.6.0/24"]
  enable_nat_gateway = true
  single_nat_gateway = true
  enable_dns_hostnames = true

  private_subnet_tags = {
    "kubernetes.io/cluster/${local.cluster_name}" = "shared"
    # EKS adds this and TF would want to remove then later
  }
}

module "eks" {
  source = "terraform-aws-modules/eks/aws"
  version = "14.0.0"

  cluster_name = var.cluster_name
  cluster_version = "1.18"
  subnets = module.vpc.private_subnets

  tags = {
    Environment = "test"
    GithubRepo = "terraform-aws-eks"
    GithubOrg = "terraform-aws-modules"
  }

  vpc_id = module.vpc.vpc_id

  worker_groups = [
    {
      name = "worker-group-2"
      instance_type = "t3.xlarge"
      additional_userdata = "echo foo bar"
      additional_security_group_ids = [
        module.eks.worker_security_group_id]
      asg_desired_capacity = 1
    },
  ]

  # GP3 isn't supported yet by the terraform module T_T
  workers_group_defaults = {
    root_volume_type = "gp2"
  }

  map_roles = var.map_roles
  map_users = var.map_users
  map_accounts = var.map_accounts
}


resource "random_id" "k8stoken_prefix" {
  byte_length = 3
}

resource "random_id" "k8stoken-suffix" {
  byte_length = 8
}

data "template_file" "node_userdata" {
  template = file("${path.module}/../install.sh")

  vars = {
    cluster_name = var.cluster_name
    cluster_region = var.region
    procri_url = var.procri_url
    kubelet_url = var.kubelet_url
    kube_proxy_url = var.kube_proxy_url
  }
}

locals {
  node_ami = var.node_ami
  cluster_name = var.cluster_name
}

resource "random_pet" "runner_name" {
  length = 2
  prefix = "mac-metal-"
  separator = "-"
}
