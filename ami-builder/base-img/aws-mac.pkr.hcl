packer {
  required_plugins {
    amazon = {
      version = ">= 1.0.0"
      source  = "github.com/hashicorp/amazon"
    }
  }
}

variable "ami_name" {
  type    = string
  default = "elotl-v1.21"
}

variable "subnet_id" {
  type    = string
  default = null
}

variable "region" {
  type    = string
  default = "us-east-1"
}

variable "elotl_kubelet_url" {
  type    = string
  default = "https://elotl-maccri.s3.amazonaws.com/kubelet-v1.21.0-alpha.1-2762-g6e049730c97"
}

variable "elotl_kube_proxy_url" {
  type    = string
  default = "https://elotl-maccri.s3.amazonaws.com/kube-proxy-v1.21.0-alpha.1-1021-g1effa3faaa6"
}

variable "elotl_procri_url" {
  type    = string
  default = "https://procri-dev.s3.amazonaws.com/procri-v0.0.1-67-gab347fd"
}

variable "xcode_version" {
  type    = string
  default = "13.2.1"
}

variable "root_volume_size_gb" {
  type    = number
  default = 150
}

locals { timestamp = regex_replace(timestamp(), "[- TZ:]", "") }

source "amazon-ebs" "mac1ebs" {
  ami_name                = "${var.ami_name}-${local.timestamp}"
  ami_virtualization_type = "hvm"
  ssh_username            = "ec2-user"
  ssh_timeout             = "2h"
  tenancy                 = "host"
  ebs_optimized           = true
  instance_type           = "mac1.metal"
  region                  = "${var.region}"
  subnet_id               = "${var.subnet_id}"
  ssh_interface           = "session_manager"
  aws_polling {
    delay_seconds = 60
    max_attempts  = 60
  }
  launch_block_device_mappings {
    device_name           = "/dev/sda1"
    volume_size           = "${var.root_volume_size_gb}"
    volume_type           = "gp3"
    iops                  = 3000
    throughput            = 125
    delete_on_termination = true
  }
  source_ami_filter {
    filters = {
      name                = "amzn-ec2-macos-11.6.*"
      root-device-type    = "ebs"
      virtualization-type = "hvm"
    }
    most_recent = true
    owners      = ["amazon"]
  }
  temporary_iam_instance_profile_policy_document {
    Version = "2012-10-17"
    # below 3 statements are inlined arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore
    # to allow connection with session manager plugin
    # see https://www.packer.io/docs/builders/amazon/ebs#iam-instance-profile-for-systems-manager
    Statement {
      Effect = "Allow"
      Action = [
        "ssm:DescribeAssociation",
        "ssm:GetDeployablePatchSnapshotForInstance",
        "ssm:GetDocument",
        "ssm:DescribeDocument",
        "ssm:GetManifest",
        "ssm:GetParameter",
        "ssm:GetParameters",
        "ssm:ListAssociations",
        "ssm:ListInstanceAssociations",
        "ssm:PutInventory",
        "ssm:PutComplianceItems",
        "ssm:PutConfigurePackageResult",
        "ssm:UpdateAssociationStatus",
        "ssm:UpdateInstanceAssociationStatus",
        "ssm:UpdateInstanceInformation"
      ]
      Resource = ["*"]
    }
    Statement {
      Effect = "Allow"
      Action = [
        "ssmmessages:CreateControlChannel",
        "ssmmessages:CreateDataChannel",
        "ssmmessages:OpenControlChannel",
        "ssmmessages:OpenDataChannel"
      ]
      Resource = ["*"]
    }
    Statement {
      Effect = "Allow"
      Action = [
        "ec2messages:AcknowledgeMessage",
        "ec2messages:DeleteMessage",
        "ec2messages:FailMessage",
        "ec2messages:GetEndpoint",
        "ec2messages:GetMessages",
        "ec2messages:SendReply"
      ]
      Resource = ["*"]
    }
  }
}

build {
  sources = ["source.amazon-ebs.mac1ebs"]
  # resize the partition to use all the space available on the EBS volume
  provisioner "shell" {
    inline = [
      "PDISK=$(diskutil list physical external | head -n1 | cut -d' ' -f1)",
      "APFSCONT=$(diskutil list physical external | grep Apple_APFS | tr -s ' ' | cut -d' ' -f8)",
      "yes | sudo diskutil repairDisk $PDISK",
      "sudo diskutil apfs resizeContainer $APFSCONT 0"
    ]
  }
  # clean the ec2-macos-init history in order to make instance from AMI as it were the first boot
  # see https://github.com/aws/ec2-macos-init#clean for details.
  provisioner "shell" {
    inline = [
      "sudo /usr/local/bin/ec2-macos-init clean --all"
    ]
  }

  # k8s node components
  provisioner "file" {
    source      = "files/com.elotl.procri.plist"
    destination = "/tmp/com.elotl.procri.plist"
  }
  provisioner "file" {
    source      = "files/com.kubernetes.kube-proxy.plist"
    destination = "/tmp/com.kubernetes.kube-proxy.plist"
  }
  provisioner "file" {
    source      = "files/com.kubernetes.kubelet.plist"
    destination = "/tmp/com.kubernetes.kubelet.plist"
  }
  provisioner "file" {
    source      = "files/kubelet-config.yaml"
    destination = "/tmp/kubelet-config.yaml"
  }

  provisioner "shell" {
    inline = [
      "sudo /usr/bin/dscl . -passwd /Users/ec2-user ec2-user",
      "sudo /usr/bin/defaults write /Library/Preferences/com.apple.loginwindow autoLoginUser ec2-user"
    ]
  }

  provisioner "shell" {
    environment_vars = [
      "AWS_DEFAULT_REGION=${var.region}",
      "PATH=/usr/bin:/bin:/usr/sbin:/sbin:/usr/local/bin:/Library/Apple/usr/bin",
    ]
    inline = [
      "brew install jq aws-iam-authenticator",

      <<EOF
sudo /System/Library/CoreServices/RemoteManagement/ARDAgent.app/Contents/Resources/kickstart \
-activate -configure -access -on \
-configure -allowAccessFor -specifiedUsers \
-configure -users ec2-user \
-configure -restart -agent -privs -all
EOF
      ,

      <<EOF
sudo /System/Library/CoreServices/RemoteManagement/ARDAgent.app/Contents/Resources/kickstart \
-configure -access -on -privs -all -users ec2-user
EOF
      ,

      # elotl stuff for EKS
      "sudo cp /tmp/com.kubernetes.kube-proxy.plist /tmp/com.kubernetes.kubelet.plist /Library/LaunchDaemons",
      "sudo -u ec2-user mkdir -p /Users/ec2-user/Library/LaunchAgents",
      "sudo -u ec2-user cp /tmp/com.elotl.procri.plist /Users/ec2-user/Library/LaunchAgents/",
      # "sudo -u ec2-user launchctl enable gui/501/com.elotl.procri",
      "sudo mkdir -p /etc/kube",
      "sudo cp /tmp/kubelet-config.yaml /etc/kube/kubelet-config.yaml",
      "curl -fsSL ${var.elotl_kubelet_url} -o /usr/local/bin/kubelet; chmod 755 /usr/local/bin/kubelet",
      "curl -fsSL ${var.elotl_kube_proxy_url} -o /usr/local/bin/kube-proxy; chmod 755 /usr/local/bin/kube-proxy",
      "curl -fsSL ${var.elotl_procri_url} -o /usr/local/bin/procri; chmod 755 /usr/local/bin/procri",
    ]
  }
}
