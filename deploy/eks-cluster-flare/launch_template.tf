# This is based on the LT that EKS would create if no custom one is specified (aws ec2 describe-launch-template-versions --launch-template-id xxx)
# there are several more options one could set but you probably dont need to modify them
# you can take the default and add your custom AMI and/or custom tags
#
# Trivia: AWS transparently creates a copy of your LaunchTemplate and actually uses that copy then for the node group. If you DONT use a custom AMI,
# then the default user-data for bootstrapping a cluster is merged in the copy.
resource "aws_launch_template" "macos" {
  name_prefix = "eks-${local.cluster_name}-worker"
  description = "k8s mac os launch template"
  update_default_version = true
  instance_type = "mac1.metal"

  monitoring {
    enabled = true
  }

  network_interfaces {
    associate_public_ip_address = true
    delete_on_termination = true
    security_groups = [
      module.eks.worker_security_group_id]
  }

  image_id = local.node_ami
  iam_instance_profile {
    name = var.iam_instance_profile
  }

  user_data = base64encode(data.template_file.node_userdata.rendered)

  tags = merge(local.k8s_cluster_tags, {
    "Name" = "${var.cluster_name}-k8s-mac-node"
  })

  placement {
    tenancy = "host"
  }

  # Supplying custom tags to EKS instances root volumes is another use-case for LaunchTemplates. (doesnt add tags to dynamically provisioned volumes via PVC tho)
  tag_specifications {
    resource_type = "volume"

    tags = merge(local.k8s_cluster_tags, {
      "Name" = "${var.cluster_name}-k8s-mac-volume"
    })
  }

  tag_specifications {
    resource_type = "instance"
    tags = merge(local.k8s_cluster_tags, {
      "Name" = "${var.cluster_name}-k8s-mac-node"
    })
  }

  lifecycle {
    create_before_destroy = true
  }
}