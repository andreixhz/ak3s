FROM rockylinux:9

RUN dnf install -y epel-release \
    && dnf update -y \
    && dnf install -y --allowerasing curl iproute net-tools openssh-server sudo \
    && curl -sfL https://get.k3s.io > /usr/local/bin/k3s \
    && chmod +x /usr/local/bin/k3s

# Run k3s without systemd
CMD ["k3s", "server", "--disable-agent"]