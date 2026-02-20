// Copyright (c) inlets Author(s) 2019. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package main

import (
	"testing"
)

func Test_MakeUserdata_InletsPro(t *testing.T) {
	userData := makeExitServerUserdata("auth", "0.7.0", "")

	wantUserdata := `#!/bin/bash
export AUTHTOKEN="auth"
export IP=$(curl -sfSL https://checkip.amazonaws.com)
export PROXY_PROTO=""

curl -SLsf https://github.com/inlets/inlets-pro/releases/download/0.7.0/inlets-pro -o /tmp/inlets-pro && \
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
`

	if userData != wantUserdata {
		t.Errorf("want: %s, but got: %s", wantUserdata, userData)
	}
}

func Test_MakeUserdata_InletsPro_WithProxyProto(t *testing.T) {
	userData := makeExitServerUserdata("auth", "0.7.0", "v1")

	wantUserdata := `#!/bin/bash
export AUTHTOKEN="auth"
export IP=$(curl -sfSL https://checkip.amazonaws.com)
export PROXY_PROTO="v1"

curl -SLsf https://github.com/inlets/inlets-pro/releases/download/0.7.0/inlets-pro -o /tmp/inlets-pro && \
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
`

	if userData != wantUserdata {
		t.Errorf("want: %s, but got: %s", wantUserdata, userData)
	}
}
