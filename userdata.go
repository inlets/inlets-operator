// Copyright (c) inlets Author(s) 2019. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package main

import "fmt"

// makeExitServerUserdata makes a user-data script in bash to setup inlets
//
//	with a systemd service and the given version. If proxyProto is non-empty,
//
// the PROXY_PROTO environment variable is set in the service configuration.
func makeExitServerUserdata(authToken, version, proxyProto string) string {
	return fmt.Sprintf(`#!/bin/bash
export AUTHTOKEN="%s"
export IP=$(curl -sfSL https://checkip.amazonaws.com)
export PROXY_PROTO="%s"

curl -SLsf https://github.com/inlets/inlets-pro/releases/download/%s/inlets-pro -o /tmp/inlets-pro && \
  chmod +x /tmp/inlets-pro  && \
  mv /tmp/inlets-pro /usr/local/bin/inlets-pro

cat > /etc/systemd/system/inlets-pro.service <<EOF
[Unit]
Description=inlets TCP server
After=network.target

[Service]
Type=simple
Restart=always
RestartSec=2
StartLimitInterval=0
EnvironmentFile=/etc/default/inlets-pro
ExecStart=/usr/local/bin/inlets-pro tcp server --auto-tls --auto-tls-san="\${IP}" --token="\${AUTHTOKEN}" --proxy-protocol="\${PROXY_PROTO}"

[Install]
WantedBy=multi-user.target
EOF

echo "AUTHTOKEN=$AUTHTOKEN" >> /etc/default/inlets-pro && \
  echo "IP=$IP" >> /etc/default/inlets-pro && \
  echo "PROXY_PROTO=$PROXY_PROTO" >> /etc/default/inlets-pro && \
  systemctl daemon-reload && \
  systemctl start inlets-pro && \
  systemctl enable inlets-pro
`, authToken, proxyProto, version)
}
