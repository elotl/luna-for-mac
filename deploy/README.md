# NO LONGER SUPPORTED

**For reference only, this installation script is not longer supported.**

We now pre-build images with the kubelet stack instead. See the [AMI
builder](../ami-builder/).

# Requirements

Terraform 0.12+ is required. Terraform 0.14+ is recommended.

# Provision EKS cluster with mac1.metal nodes

`cp example.tfvars.json.example terraform.tfvars.json`
Change value of variables in `terraform.tfvars.json`

Use `terraform init` & `terraform apply` to add mac1.metal nodes to your EKS cluster.

## Terraform variables:

| Name | Description | Example |
| ---- | ----------- | ------- |
| cluster_name | Name of existing EKS cluster (required) | my-cluster |
| iam_instance_profile | AWS instance profile with required permissions | arn:aws:iam::689494258501:instance-profile/MacMetal1EKSWorkerNodeRole |
| region | AWS region of the cluster (required) | us-west-2 |
| node_ami | base AMI for mac nodes, k8s stack will be installed on top (required) | ami-234sdf23 |
| kubelet_version | Url of kubelet binary, compiled for darwin (required) | https://elotl-maccri.s3.amazonaws.com/kubelet-v1.18.15-38-g985cf1be960 |
| kube_proxy_url | Url of kube-proxy binary, compiled for darwin (required) | https://elotl-maccri.s3.amazonaws.com/kube-proxy-darwin |
| procri_url | Url of proCRI binary (required) |  https://procri-dev.s3.amazonaws.com/procri-v0.0.1-9-g818f3a2 |
| nodes_number | Number of mac1.metal nodes to provision | 1 |
terraform plan -var 'cluster_name=flare-test' -var 'node_ami=ami-026383ea21564b248

## Customer infra configs

1. [Flare EKS cluster with mac1.metal nodes](eks-cluster-flare)
