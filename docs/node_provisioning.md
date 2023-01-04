# Adding mac1.metal to EKS cluster
1. Start mac1.metal instance with a proper AMI (mac-k8s-node-v*).
2. EC2 instance has to be have `MacMetal1EKSWorkerNodeRole` IAM role attached.
3. EC2 instance has to be in one of the EKS cluster VPC subnet (and subnet has to have `kubernetes.io/cluster/<cluster-name>: shared` tag)
4. EC2 instance has to have tag: `kubernetes.io/cluster/<cluster-name>: owned`
5. [aws-iam-authenticator](https://github.com/kubernetes-sigs/aws-iam-authenticator) has to be installed on the instance. AMI already has it.
6. run `install.sh` from `procri/release/install.sh`
7. Edit `configmap/aws-auth` in `kube-system` namespace and add:
```yaml
apiVersion: v1
data:
  mapRoles: |
    - groups:
      - system:bootstrappers
      - system:nodes
      rolearn: arn:aws:iam::689494258501:role/MacMetal1EKSWorkerNodeRole
      username: system:node:{{EC2PrivateDNSName}}
```

Useful links
1. [Medium post](https://medium.com/@tarunprakash/5-things-you-need-know-to-add-worker-nodes-in-the-aws-eks-cluster-bfbcb9fa0c37)
2. [AWS docs](https://aws.amazon.com/premiumsupport/knowledge-center/eks-worker-nodes-cluster/)

We should automate as much of this as possible in `install.sh` script.
