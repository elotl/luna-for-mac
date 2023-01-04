# This is based on the LT that EKS would create if no custom one is specified (aws ec2 describe-launch-template-versions --launch-template-id xxx)
# there are several more options one could set but you probably dont need to modify them
# you can take the default and add your custom AMI and/or custom tags
#
# Trivia: AWS transparently creates a copy of your LaunchTemplate and actually uses that copy then for the node group. If you DONT use a custom AMI,
# then the default user-data for bootstrapping a cluster is merged in the copy.
locals {
  cluster_name             = "henry-systest"
  cluster_region           = "us-east-1"
  vpc_private_subnet       = "subnet-0deb91d7eca748c81"
  worker_security_group_id = "sg-0630c4ccaec2e291b"
  instance_profile_name    = "henry-systest20211126201345979800000011"
  ami_id                   = "ami-0e99d916b717e9a3c"
}

resource "null_resource" "macos_user_data" {
  provisioner "local-exec" {
    command = "sh -ex ./generate_user_data.sh ${local.cluster_name} ${local.cluster_region} > user_data.sh"
  }
}

data "local_file" "macos_user_data" {
  filename = "${path.module}/user_data.sh"

  depends_on = [null_resource.macos_user_data]
}

resource "aws_launch_template" "macos" {
  name_prefix            = "eks-${local.cluster_name}-mac"
  description            = "Kubernetes macOs launch template"
  update_default_version = true
  ebs_optimized          = true
  instance_type          = "mac1.metal"
  image_id               = local.ami_id

  user_data = data.local_file.macos_user_data.content_base64

  monitoring {
    enabled = true
  }

  network_interfaces {
    associate_public_ip_address = false
    subnet_id                   = local.vpc_private_subnet
    security_groups             = [local.worker_security_group_id]
  }

  iam_instance_profile {
    name = local.instance_profile_name
  }

  tags = {
    "Name"                                        = "${local.cluster_name}-kubernetes-mac-node"
    "kubernetes.io/cluster/${local.cluster_name}" = "owned"
  }

  placement {
    tenancy = "host"
  }

  # Supplying custom tags to EKS instances root volumes is another use-case for LaunchTemplates. (doesnt add tags to dynamically provisioned volumes via PVC tho)
  tag_specifications {
    resource_type = "volume"
    tags = {
      "Name"                                        = "${local.cluster_name}-k8s-mac-volume"
      "kubernetes.io/cluster/${local.cluster_name}" = "owned"
    }
  }

  tag_specifications {
    resource_type = "instance"
    tags = {
      "Name"                                        = "${local.cluster_name}-k8s-mac-node"
      "kubernetes.io/cluster/${local.cluster_name}" = "owned"
    }
  }

  lifecycle {
    create_before_destroy = true
  }
}
