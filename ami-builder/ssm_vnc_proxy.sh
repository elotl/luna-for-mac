#!/bin/sh

aws ssm start-session \
--document-name AWS-StartPortForwardingSession \
--parameters '{"portNumber":["5900"],"localPortNumber":["5999"]}' \
--target $@
