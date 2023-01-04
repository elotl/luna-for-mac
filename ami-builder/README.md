# AMI packer builder for Mac Intel Kubernetes nodes

building base AMI (with xCode, etc.) takes a lot of time. It would be good
to have a way to add Elotl's stack and Buildkite / Flare / CircleCI / any
customer's specific software on top, without rebuilding base AMI.

# Running packer

To build the Mac image, simple run:

	$ packer build .

# Activating the launchtcl daemon after `packer build`

After the packer build is done, there’s still one thing left to do: enable the launch
daemon for the ec2-user when the mac node starts.

We need to do this via the macOS’s UI, we can’t do this via packer, or SSH.
That’s because macOS has a sandbox to prevent unauthorized changes to happen
outside the UI, it’s an additional layer of security.

To enable the procri launch daemon, you’ll need to:

1. Create a tunnel between your workstation and the mac node with [SSM][],
2. Connect to the mac node with VNC through the SSM tunnel
3. Enable the procri launch agent via the Terminal app

[SSM]: https://docs.aws.amazon.com/systems-manager/latest/userguide/what-is-systems-manager.html

To create a tunnel between your workstation and the mac node, you’ll need:

1. The AWS region where the node runs
2. The instance ID of the node

Run the `ssm_vnc_proxy.sh` script in this directory like this to create the
tunnel:

	$ env AWS_DEFAULT_REGION=<region> ./ssm_vnc_proxy.sh <instance id>

This will create a local listener on port 5999. You can use this to connect to
the node’s VNC port (5900).

Then connect to the node via VNC through the tunnel. You can use something like
Remmina on Linux, or the Share Screen app on macOS. Connect to localhost:5999
with the username/password: ec2-user/ec2-user. Then you should see the macOS
login screen for the node. Log into it with the same username/password:
ec2-user/ec2-user.

Once you are logged into macOS. Start a terminal and execute the following:

    $ launchctl enable gui/501/com.elotl.procri

Then create a new AMI via the EC2 console.
