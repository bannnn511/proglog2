#!/bin/sh


ID=$(echo $HOSTNAME | rev | cut -d- -f1 | rev)
cat > /var/run/proglog/config.yaml <<EOD
data-dir: /vat/run/prolog/config.yaml
rpc-port: $1
bind-addr: "$2.svc.cluster.local:$3"
 $([ $ID != 0 ] && echo "start-join-addrs: \
                "proglog-0.proglog.$2.svc.cluster.local:$3"")
bootstrap: $([ $ID = 0 ] && echo true || echo false )
EOD